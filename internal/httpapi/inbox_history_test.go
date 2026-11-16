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
	"sync/atomic"
	"testing"
	"time"
)

func TestHistorySkipsPreInvitationMessagesAndNeverSends(t *testing.T) {
	ctx := context.Background()
	db := testutil.Mongo(t)
	now := time.Now().UTC()
	sent := now.Add(-time.Hour)
	s := &server{db: db, secret: []byte(strings.Repeat("x", 48))}
	u := core.User{ID: "owner", Email: "owner@example.test", Role: "organizer"}
	db.Collection("users").InsertOne(ctx, u)
	e := core.Event{ID: "event", Kind: "general", OwnerID: u.ID, PaymentStatus: "paid", PublishedAt: &sent}
	db.Collection("events").InsertOne(ctx, e)
	g := core.Guest{ID: "guest", EventID: e.ID, Phone: "+51999111222", SentAt: &sent}
	db.Collection("guests").InsertOne(ctx, g)
	var outbound atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			outbound.Add(1)
		}
		reply(w, 200, []map[string]any{{"id": "recent", "from": "51999111222@c.us", "body": "No pude responder la encuesta", "timestamp": now.Unix()}, {"id": "old", "from": "51999111222@c.us", "body": "Mensaje anterior a la invitación", "timestamp": now.Add(-24 * time.Hour).Unix()}})
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	t.Setenv("WAZEND_GENERAL_BASE_URL", provider.URL)
	t.Setenv("WAZEND_GENERAL_SESSION", "session")
	t.Setenv("WAZEND_GENERAL_API_KEY", "test")
	token, _ := s.sign(u, "password")
	for i := range 2 {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
		r.SetPathValue("id", e.ID)
		r.SetPathValue("guestId", g.ID)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.conversationHistory(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result struct {
			Added int `json:"added"`
		}
		json.Unmarshal(w.Body.Bytes(), &result)
		expected := 1
		if i == 1 {
			expected = 0
		}
		if result.Added != expected {
			t.Fatal("history replay duplicated or imported old messages", w.Body.String())
		}
	}
	if outbound.Load() != 0 {
		t.Fatal("history caused an outbound message")
	}
	count, _ := db.Collection("inboxMessages").CountDocuments(ctx, bson.M{"eventId": e.ID})
	if count != 1 {
		t.Fatal("unexpected history count", count)
	}
}
