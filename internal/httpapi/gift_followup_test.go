package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
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

func TestGiftFollowupOnlyAfterYesAndOnceAcrossWorkers(t *testing.T) {
	for _, kind := range []string{"wedding", "general"} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.Mongo(t)
			ctx := context.Background()
			now := time.Now().UTC()
			e := core.Event{ID: "event", Kind: kind, Title: "Celebración", Slug: "celebracion", Capacity: 10, PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(24 * time.Hour), MaybeHoldHours: 48, ResponseVersion: 1, Responses: map[string]core.Response{}}
			g := core.Guest{ID: "guest", EventID: e.ID, Name: "Ana", Phone: "+51999111222", Seats: 1, InviteToken: strings.Repeat("a", 48)}
			if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Collection("guests").InsertOne(ctx, g); err != nil {
				t.Fatal(err)
			}
			var count atomic.Int32
			provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var message map[string]any
				if json.NewDecoder(r.Body).Decode(&message) != nil {
					t.Error("invalid message")
				}
				host, _ := core.InvitationHost(kind, e.Slug)
				if message["chatId"] != "51999111222@c.us" || !strings.Contains(message["text"].(string), "https://"+host+core.GuestInvitationPath(g)+"/regalos") {
					t.Error("wrong personal gift link")
				}
				count.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"gift-message"}`))
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
			t.Setenv(prefix+"SESSION", "test")
			s := &server{db: db}
			repo := mongostore.Responses{DB: db}
			apply := func(choice string) {
				t.Helper()
				if err := s.responses().Apply(ctx, e.ID, g, choice, "Esperando permiso", ""); err != nil {
					t.Fatal(err)
				}
			}
			apply("maybe")
			s.giftFollowupStep(ctx)
			if count.Load() != 0 {
				t.Fatal("sent for maybe")
			}
			apply("going")
			// A newer negative answer suppresses an already queued message.
			snapshot, err := repo.Snapshot(ctx, e.ID)
			if err != nil {
				t.Fatal(err)
			}
			if ok, err := repo.CommitManagedResponse(ctx, snapshot, g.ID, core.Response{Choice: "not_going"}); err != nil || !ok {
				t.Fatal(err)
			}
			s.giftFollowupStep(ctx)
			if count.Load() != 0 {
				t.Fatal("sent after decline")
			}
			snapshot, _ = repo.Snapshot(ctx, e.ID)
			if ok, err := repo.CommitManagedResponse(ctx, snapshot, g.ID, core.Response{Choice: "going", Seats: 1}); err != nil || !ok {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); s.giftFollowupStep(ctx) }()
			}
			wg.Wait()
			s.giftFollowupStep(ctx)
			if count.Load() != 1 {
				t.Fatalf("sent %d messages", count.Load())
			}
			var saved core.Event
			if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved); err != nil {
				t.Fatal(err)
			}
			if saved.GiftFollowups[g.ID] != "sent" {
				t.Fatal(saved.GiftFollowups)
			}
		})
	}
}

func TestGiftFollowupWebConfirmationQueuesWithoutLogin(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "web", Kind: "general", Capacity: 3, PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(24 * time.Hour), MaybeHoldHours: 48}
	g := core.Guest{ID: "guest", EventID: e.ID, Name: "Ana", Seats: 1, InviteToken: strings.Repeat("b", 48)}
	_, _ = db.Collection("events").InsertOne(ctx, e)
	_, _ = db.Collection("guests").InsertOne(ctx, g)
	s := &server{db: db}
	for _, choice := range []string{"maybe", "going"} {
		r := httptest.NewRequest("POST", "/public/rsvp/"+g.InviteToken, strings.NewReader(`{"response":"`+choice+`","reason":"Esperando permiso","companions":[]}`))
		r.SetPathValue("token", g.InviteToken)
		w := httptest.NewRecorder()
		s.rsvp(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		saved, err := (mongostore.Responses{DB: db}).Snapshot(ctx, e.ID)
		if err != nil {
			t.Fatal(err)
		}
		want := ""
		if choice == "going" {
			want = "pending"
		}
		if saved.GiftFollowups[g.ID] != want {
			t.Fatal(saved.GiftFollowups)
		}
	}
}
