package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestReinvitedGuestReceivesGiftListAgainAfterYesDespiteMessages(t *testing.T) {
	ctx, now := context.Background(), time.Now().UTC()
	db := testutil.Mongo(t)
	s := &server{db: db, product: "general"}
	e := core.Event{ID: "birthday", Kind: "general", Title: "Cumpleaños", Slug: "cumpleanos", Capacity: 10, PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(24 * time.Hour), ResponseVersion: 1, ResponseReceipts: map[string]bool{}, Wishlist: core.Wishlist{Version: 1, Items: []core.Gift{{ID: "gift", Title: "Regalo"}}}, Responses: map[string]core.Response{"guest": {Choice: "going", Seats: 1}}, GiftFollowups: map[string]string{"guest": "sent"}}
	g := core.Guest{ID: "guest", EventID: e.ID, Phone: "+51999111222", Seats: 1, InviteToken: strings.Repeat("a", 48), SentAt: &now}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("guests").InsertOne(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := (mongostore.GuestManagement{Responses: mongostore.Responses{DB: db}}).Remove(ctx, e.ID, g.ID, now); err != nil {
		t.Fatal(err)
	}
	g.InviteToken = strings.Repeat("b", 48)
	if _, err := db.Collection("guests").InsertOne(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := s.responses().ApplyProvider(ctx, e.ID, g, "going", "new-confirmation", now.Unix()); err != nil {
		t.Fatal(err)
	}
	var queued core.Event
	db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&queued)
	if queued.GiftFollowups[g.ID] != "pending" {
		t.Fatal("old guest send suppressed the new gift list", queued.GiftFollowups)
	}
	sends := 0
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		text, _ := payload["text"].(string)
		if r.URL.Path != "/api/sendText" || !strings.Contains(text, "/regalos") {
			t.Error("gift list not sent")
		}
		sends++
		reply(w, 200, map[string]string{"id": "gift-confirmation"})
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	t.Setenv("WAZEND_GENERAL_BASE_URL", provider.URL)
	t.Setenv("WAZEND_GENERAL_API_KEY", "test")
	t.Setenv("WAZEND_GENERAL_SESSION", "shared")
	m := core.InboxMessage{ID: "after-yes", ProviderID: "after-yes", Phone: g.Phone, Session: "shared", Text: "Hola", Direction: "in", CreatedAt: now, ReceivedAt: now}
	if err := routeIncoming(ctx, []*server{s}, m); err != nil {
		t.Fatal(err)
	}
	s.giftFollowupStep(ctx, e.ID)
	s.giftFollowupStep(ctx, e.ID)
	if sends != 1 {
		t.Fatal("gift list was suppressed or duplicated", sends)
	}
}
