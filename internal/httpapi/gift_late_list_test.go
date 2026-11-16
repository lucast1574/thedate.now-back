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

func TestConfirmedGuestReceivesListSavedAfterYesOrMaybe(t *testing.T) {
	for _, kind := range []string{"general", "wedding"} {
		for _, maybe := range []bool{false, true} {
			t.Run(kind+"/"+map[bool]string{false: "yes", true: "maybe-to-yes"}[maybe], func(t *testing.T) {
				ctx, now := context.Background(), time.Now().UTC()
				db := testutil.Mongo(t)
				s := &server{db: db, product: kind}
				e := core.Event{ID: "event", Kind: kind, Slug: "celebracion", Capacity: 10, PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(24 * time.Hour), MaybeHoldHours: 48, ResponseVersion: 1, ResponseReceipts: map[string]bool{}, Responses: map[string]core.Response{}, Wishlist: core.Wishlist{Version: 1}}
				g := core.Guest{ID: "guest", EventID: e.ID, Seats: 1, Phone: "+51999111222", InviteToken: strings.Repeat("a", 48)}
				if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Collection("guests").InsertOne(ctx, g); err != nil {
					t.Fatal(err)
				}
				if maybe {
					if err := s.responses().Apply(ctx, e.ID, g, "maybe", "Esperando permiso", ""); err != nil {
						t.Fatal(err)
					}
				}
				if err := s.responses().ApplyProvider(ctx, e.ID, g, "going", "yes", 100); err != nil {
					t.Fatal(err)
				}
				var confirmed core.Event
				db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&confirmed)
				if confirmed.GiftFollowups[g.ID] != "pending" {
					t.Fatal("confirmation discarded before list was ready")
				}
				sends := 0
				provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					json.NewDecoder(r.Body).Decode(&payload)
					text, _ := payload["text"].(string)
					if !strings.Contains(text, "/regalos") {
						t.Error("missing gift link")
					}
					sends++
					reply(w, 200, map[string]string{"id": "gift-message"})
				}))
				defer provider.Close()
				previous := wazendClient
				wazendClient = provider.Client()
				defer func() { wazendClient = previous }()
				prefix := "WAZEND_GENERAL_"
				if kind == "wedding" {
					prefix = "WAZEND_WEDDING_"
				}
				t.Setenv(prefix+"BASE_URL", provider.URL)
				t.Setenv(prefix+"API_KEY", "test")
				t.Setenv(prefix+"SESSION", "shared")
				s.giftFollowupStep(ctx, e.ID)
				if sends != 0 {
					t.Fatal("empty list sent")
				}
				// Cover confirmations created before this fix as well as new pending ones.
				if maybe {
					db.Collection("events").UpdateOne(ctx, bson.M{"_id": e.ID}, bson.M{"$unset": bson.M{"giftFollowups": ""}})
				}
				next := core.Wishlist{Version: 2, Items: []core.Gift{{ID: "gift", Title: "Regalo", URL: "https://example.test/regalo"}}}
				if ok, err := (mongostore.Wishlists{DB: db}).Commit(ctx, e.ID, 1, next, false); err != nil || !ok {
					t.Fatal(ok, err)
				}
				repo := mongostore.Responses{DB: db}
				for range 2 {
					if err := repo.QueueMissingGiftFollowups(ctx, e.ID, now); err != nil {
						t.Fatal(err)
					}
					s.giftFollowupStep(ctx, e.ID)
				}
				if sends != 1 {
					t.Fatal("late list lost or duplicated", sends)
				}
			})
		}
	}
}
