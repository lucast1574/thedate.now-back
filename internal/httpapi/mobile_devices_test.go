package httpapi

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeMobileSender struct {
	mu    sync.Mutex
	count int
	fail  bool
}

func (f *fakeMobileSender) Send(context.Context, string, core.MobileChange) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	if f.fail {
		return false, errors.New("unknown")
	}
	return false, nil
}
func TestMobileRegistrationIsolationRevocationAndConcurrentDelivery(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	sender := &fakeMobileSender{}
	s := &server{db: db, product: "general", secret: []byte(strings.Repeat("s", 32)), mobileSender: sender}
	u := core.User{ID: "user", Role: "user", Email: "u@example.test"}
	other := core.User{ID: "other", Role: "user", Email: "o@example.test"}
	db.Collection("users").InsertMany(ctx, []any{u, other})
	token, _ := s.sign(u, "password")
	otherToken, _ := s.sign(other, "password")
	id := "bb804d3a-5bb6-45c0-b69e-4b4ec3a87d26"
	call := func(method, path, token, body string) int {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w.Code
	}
	body := `{"installationId":"` + id + `","fid":"abcdefghijklmnopqrstuv","platform":"android"}`
	if call("POST", "/devices", "", body) != 401 || call("POST", "/devices", token, body) != 200 {
		t.Fatal("registration auth")
	}
	var d core.MobileDevice
	load := func() {
		if err := db.Collection("mobileDevices").FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
			t.Fatal(err)
		}
	}
	load()
	baseline, binding := d.RegisteredAt, d.Binding
	if call("POST", "/devices", token, body) != 200 {
		t.Fatal("refresh")
	}
	load()
	if !d.RegisteredAt.Equal(baseline) || d.Binding != binding {
		t.Fatal("heartbeat reset baseline")
	}
	if call("DELETE", "/devices/"+id, otherToken, "") != 204 {
		t.Fatal("foreign delete")
	}
	load()
	now := time.Now().UTC().Add(time.Second)
	e := core.Event{ID: "event", Kind: "general", OwnerID: other.ID, CoupleUserIDs: []string{u.ID}, PaymentStatus: "paid", Responses: map[string]core.Response{"guest": {Choice: "going", RespondedAt: now}}}
	db.Collection("events").InsertOne(ctx, e)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.deliverMobile(ctx, d); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if sender.count != 1 {
		t.Fatalf("duplicate sends: %d", sender.count)
	}
	// A second change is claimed once even if the provider outcome is unknown.
	now = now.Add(time.Second)
	e.Responses["guest"] = core.Response{RespondedAt: now}
	db.Collection("events").UpdateOne(ctx, bson.M{"_id": e.ID}, bson.M{"$set": bson.M{"responses": e.Responses}})
	sender.fail = true
	if s.deliverMobile(ctx, d) == nil {
		t.Fatal("unknown not reported")
	}
	s.deliverMobile(ctx, d)
	if sender.count != 2 {
		t.Fatal("unknown replayed")
	}
	db.Collection("events").UpdateOne(ctx, bson.M{"_id": e.ID}, bson.M{"$set": bson.M{"coupleUserIds": []string{}}})
	current, err := (mongostore.MobileDevices{DB: db}).Current(ctx, d, e.ID)
	if err != nil || current {
		t.Fatal("revoked access")
	}
	if call("POST", "/devices", otherToken, body) != 200 {
		t.Fatal("switch account")
	}
	load()
	if d.Binding == binding || d.UserID != other.ID {
		t.Fatal("old binding retained")
	}
	if err := (mongostore.MobileDevices{DB: db}).RemoveBinding(ctx, core.MobileDevice{ID: id, Binding: binding}); err != nil {
		t.Fatal(err)
	}
	load()
	// Product routes do not accept sessions issued by the other app.
	wedding := &server{db: db, product: "wedding", secret: productSecret(s.secret, "wedding"), mobileSender: sender}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/devices", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	wedding.routes().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("cross product session accepted")
	}
	if call("DELETE", "/devices/"+id, otherToken, "") != 204 {
		t.Fatal("logout delete")
	}
	if err := db.Collection("mobileDevices").FindOne(ctx, bson.M{"_id": id}).Err(); err != mongo.ErrNoDocuments {
		t.Fatal("device retained")
	}
}

func TestBrowserNotificationsFollowNewUnreadMessages(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	s := &server{db: db, product: "general", secret: []byte(strings.Repeat("s", 32))}
	user := core.User{ID: "owner", Email: "test@example.test", Role: "user"}
	event := core.Event{ID: "event", Kind: s.product, OwnerID: user.ID, PaymentStatus: "paid"}
	if _, err := db.Collection("users").InsertOne(ctx, user); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("events").InsertOne(ctx, event); err != nil {
		t.Fatal(err)
	}
	inbox := mongostore.Inbox{DB: db}
	for id, at := range map[string]time.Time{"old": now.Add(-time.Minute), "new": now, "read": now} {
		if _, err := inbox.Put(ctx, core.InboxMessage{
			ID: id, EventID: event.ID, GuestID: id, Direction: "in", CreatedAt: at, ReceivedAt: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := inbox.Mark(ctx, event.ID, "read", "read", nil); err != nil {
		t.Fatal(err)
	}
	changes, err := s.notificationChanges(ctx, core.MobileDevice{Platform: "web"}, event, now.Add(-time.Second))
	if err != nil || len(changes) != 1 || changes[0].ID != "new" || changes[0].Screen != "messages" {
		t.Fatalf("unexpected browser changes: %+v, error %v", changes, err)
	}
	if err := inbox.Mark(ctx, event.ID, "new", "new", nil); err != nil {
		t.Fatal(err)
	}
	changes, err = s.notificationChanges(ctx, core.MobileDevice{Platform: "web"}, event, now.Add(-time.Second))
	if err != nil || len(changes) != 0 {
		t.Fatalf("read messages were included: %+v, error %v", changes, err)
	}
	token, err := s.sign(user, "password")
	if err != nil {
		t.Fatal(err)
	}
	checkSummary := func(expected string) {
		t.Helper()
		r := httptest.NewRequest("GET", "/events/event/conversations?summary=unread", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		if w.Code != 200 || strings.TrimSpace(w.Body.String()) != expected {
			t.Fatalf("unexpected unread summary: %d %s", w.Code, w.Body.String())
		}
	}
	checkSummary(`{"unread":true}`)
	if err := inbox.Mark(ctx, event.ID, "old", "old", nil); err != nil {
		t.Fatal(err)
	}
	checkSummary(`{"unread":false}`)
}
