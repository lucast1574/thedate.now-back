package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestDeliveryKeepsFileOrderAppendsManualAndWaitsForWebsiteAndCapacity(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	u := core.User{ID: "host", Email: "host@example.test", Role: "organizer"}
	_, _ = db.Collection("users").InsertOne(ctx, u)
	e := core.Event{ID: "event", Kind: "general", Slug: "party", OwnerID: u.ID, PaymentStatus: "paid", Capacity: 3, StartAt: now.Add(time.Hour), MaybeHoldHours: 48}
	_, _ = db.Collection("events").InsertOne(ctx, e)
	var ready atomic.Bool
	var mu sync.Mutex
	var names []string
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/sessions/test" {
			_ = json.NewEncoder(w).Encode(map[string]string{"name": "test", "status": "WORKING"})
			return
		}
		var input struct {
			ChatID string `json:"chatId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&input)
		mu.Lock()
		names = append(names, input.ChatID)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "message-" + input.ChatID})
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	for key, value := range map[string]string{"WAZEND_BASE_URL": provider.URL, "WAZEND_API_KEY": "test", "WAZEND_SESSION": "test", "DOKPLOY_URL": provider.URL, "DOKPLOY_THE_DATE_API_KEY": "test", "DOKPLOY_INVITATION_IMAGE": "registry.example.test/invitation@sha256:" + strings.Repeat("a", 64)} {
		t.Setenv(key, value)
	}
	s := &server{db: db, secret: []byte(strings.Repeat("s", 32)), invitationReady: func(context.Context, core.Event) bool { return ready.Load() }}
	token, _ := s.sign(u, "password")
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w
	}
	w := call("POST", "/events/event/guests/import", `{"guests":[{"name":"First","phone":"+51911111111","seats":2},{"name":"Second","phone":"+51922222222","seats":1}]}`)
	if w.Code != 200 {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}
	w = call("POST", "/events/event/guests", `{"name":"Manual","phone":"+51933333333","seats":1}`)
	if w.Code != 201 {
		t.Fatalf("manual: %d %s", w.Code, w.Body.String())
	}
	w = call("POST", "/events/event/delivery", "")
	if w.Code != 202 {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	s.invitationStep(ctx)
	if len(names) != 0 {
		t.Fatal("sent before deployment")
	}
	_, _ = db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"publishedAt": now}})
	_, _ = db.Collection("deployments").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"phase": "ready"}})
	s.invitationStep(ctx)
	if len(names) != 0 {
		t.Fatal("sent before public website verification")
	}
	ready.Store(true)
	s.invitationStep(ctx)
	if len(names) != 2 || names[0] != "51911111111@c.us" || names[1] != "51922222222@c.us" {
		t.Fatalf("wrong send order or exceeded capacity: %v", names)
	}
	cur, err := db.Collection("guests").Find(ctx, bson.M{"eventId": e.ID}, options.Find().SetSort(mongostore.GuestQueueSort()))
	if err != nil {
		t.Fatal(err)
	}
	var guests []core.Guest
	if err = cur.All(ctx, &guests); err != nil {
		t.Fatal(err)
	}
	if len(guests) != 3 || guests[2].Name != "Manual" || guests[0].QueuePosition >= guests[1].QueuePosition || guests[1].QueuePosition >= guests[2].QueuePosition {
		t.Fatal("manual guest was not appended")
	}
	if err = s.responses().Apply(ctx, e.ID, guests[0], "not_going", "", "decline-receipt"); err != nil {
		t.Fatal(err)
	}
	s.invitationStep(ctx)
	if len(names) != 3 || names[2] != "51933333333@c.us" {
		t.Fatalf("next guest was not invited automatically: %v", names)
	}
	s.invitationStep(ctx)
	if len(names) != 3 {
		t.Fatal("duplicate automatic send")
	}
}

func TestUnknownSendRetainsClaimAndSeatsWithoutRetry(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "e", Kind: "general", PaymentStatus: "paid", PublishedAt: &now, Capacity: 1, StartAt: now.Add(time.Hour)}
	_, _ = db.Collection("events").InsertOne(ctx, e)
	for _, g := range []core.Guest{{ID: "one", EventID: e.ID, Name: "First", Seats: 1, QueuePosition: 1, Response: "pending"}, {ID: "two", EventID: e.ID, Name: "Second", Seats: 1, QueuePosition: 2, Response: "pending"}} {
		_, _ = db.Collection("guests").InsertOne(ctx, g)
	}
	var calls atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/sessions/test" {
			_ = json.NewEncoder(w).Encode(map[string]string{"name": "test", "status": "WORKING"})
			return
		}
		calls.Add(1)
		w.WriteHeader(500)
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	t.Setenv("WAZEND_BASE_URL", provider.URL)
	t.Setenv("WAZEND_API_KEY", "test")
	t.Setenv("WAZEND_SESSION", "test")
	s := &server{db: db, invitationReady: func(context.Context, core.Event) bool { return true }}
	s.dispatchInvitations(ctx, e)
	s.dispatchInvitations(ctx, e)
	if calls.Load() != 1 {
		t.Fatalf("unknown result was retried or capacity exceeded: %d", calls.Load())
	}
	snapshot, err := (mongostore.Responses{DB: db}).Snapshot(ctx, e.ID)
	if err != nil || core.Occupied(snapshot, now) != 1 {
		t.Fatal("uncertain send lost its reservation")
	}
}
