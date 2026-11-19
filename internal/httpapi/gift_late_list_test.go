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
)

func TestConfirmedGuestReceivesEmptyListAndOneFirstGiftNotice(t *testing.T) {
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
				if confirmed.GiftFollowups[g.ID] != "pending_link" {
					t.Fatal("confirmation discarded before list was ready")
				}
				var sends atomic.Int32
				provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					json.NewDecoder(r.Body).Decode(&payload)
					text, _ := payload["text"].(string)
					count := sends.Add(1)
					if !strings.Contains(text, "/regalos") || strings.Contains(text, "ya tiene regalos") != (count == 2) {
						t.Error("wrong gift message or missing link")
					}
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
				if sends.Load() != 1 {
					t.Fatal("empty list link was not sent")
				}
				w := httptest.NewRecorder()
				s.routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/public/wishlist/"+g.InviteToken, nil))
				if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
					t.Fatal(w.Code, w.Body.String())
				}
				repo := mongostore.Responses{DB: db}
				old := e.Wishlist
				gift := core.Gift{ID: "gift", Title: "Regalo", URL: "https://example.test/regalo"}
				other := core.Gift{ID: "other", Title: "Otro regalo", URL: "https://example.test/otro"}
				for _, items := range [][]core.Gift{{gift}, {gift, other}, {}, {other}} {
					next, err := core.EditWishlist(old, core.Wishlist{Items: items})
					if err != nil {
						t.Fatal(err)
					}
					if ok, err := (mongostore.Wishlists{DB: db}).Commit(ctx, e.ID, old.Version, next, false); err != nil || !ok {
						t.Fatal(ok, err)
					}
					old = next
					if err := repo.QueueMissingGiftFollowups(ctx, e.ID, now); err != nil {
						t.Fatal(err)
					}
					var workers sync.WaitGroup
					for range 3 {
						workers.Go(func() { s.giftFollowupStep(ctx, e.ID) })
					}
					workers.Wait()
					if sends.Load() != 2 {
						t.Fatal("first gift notice lost or duplicated", sends.Load())
					}
				}
			})
		}
	}
}
