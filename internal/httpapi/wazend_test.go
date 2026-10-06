package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentSendsClaimGuestBeforeProviderCall(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	u := core.User{ID: "owner", Email: "host@gmail.com", Role: "organizer"}
	_, err := db.Collection("users").InsertOne(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	e := core.Event{ID: "event", Kind: "general", Slug: "party", OwnerID: u.ID, PaymentStatus: "paid", PublishedAt: &now, StartAt: now}
	_, err = db.Collection("events").InsertOne(ctx, e)
	if err != nil {
		t.Fatal(err)
	}
	g := core.Guest{ID: "guest", EventID: e.ID, Phone: "+51999999999", Name: "Guest", Seats: 1, InviteToken: strings.Repeat("a", 48)}
	_, err = db.Collection("guests").InsertOne(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	var sends atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		time.Sleep(20 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "message-1"})
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	t.Setenv("WAZEND_BASE_URL", provider.URL)
	t.Setenv("WAZEND_API_KEY", "test")
	t.Setenv("WAZEND_SESSION", "test")
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, err := s.sign(u, "password")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/events/event/send-invitations", nil)
			r.SetPathValue("id", e.ID)
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.sendInvitations(w, r)
			if w.Code != 200 {
				t.Errorf("send: %d %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	if sends.Load() != 1 {
		t.Fatalf("duplicate sends: %d", sends.Load())
	}
	if err = db.Collection("guests").FindOne(ctx, bson.M{"_id": g.ID}).Decode(&g); err != nil {
		t.Fatal(err)
	}
	if g.SentAt == nil || g.InvitationMessageID != "message-1" {
		t.Fatal("send checkpoint missing")
	}
}
func TestHTTPRejectsExtraJSONAndUntrustedOrigin(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"email":"a@gmail.com"} {"email":"b@gmail.com"}`))
	var in credentials
	if decode(r, &in) == nil {
		t.Fatal("multiple JSON objects accepted")
	}
	called := false
	handler := cors(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	r = httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Origin", "https://crea.thedate.now.evil.test")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 || called {
		t.Fatal("untrusted origin reached handler")
	}
}
