package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRejectedPollRetryDoesNotRepeatTheCouplesMessage(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now()
	e := core.Event{ID: "e", Kind: "wedding", PaymentStatus: "paid", PublishedAt: &now, Capacity: 2}
	g := core.Guest{ID: "g", EventID: e.ID, Seats: 1}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("guests").InsertOne(ctx, g); err != nil {
		t.Fatal(err)
	}
	var texts, polls atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sendText" {
			texts.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "text-1"})
			return
		}
		if r.URL.Path == "/api/sendPoll" {
			if polls.Add(1) == 1 {
				w.WriteHeader(400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "poll-1"})
			return
		}
		w.WriteHeader(404)
	}))
	defer provider.Close()
	old := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = old }()
	t.Setenv("WAZEND_BASE_URL", provider.URL)
	t.Setenv("WAZEND_SESSION", "test")
	s := &server{db: db}
	s.sendOneInvitation(ctx, e, g, provider.URL+"/api/sendPoll", "test")
	if err := db.Collection("guests").FindOne(ctx, bson.M{"_id": g.ID}).Decode(&g); err != nil {
		t.Fatal(err)
	}
	if g.SendState != "failed" || g.InvitationTextMessageID != "text-1" {
		t.Fatal("partial send checkpoint missing")
	}
	if err := s.retryRejectedInvitations(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	s.sendOneInvitation(ctx, e, g, provider.URL+"/api/sendPoll", "test")
	if texts.Load() != 1 || polls.Load() != 2 {
		t.Fatal("retry duplicated the message")
	}
	if err := db.Collection("guests").FindOne(ctx, bson.M{"_id": g.ID}).Decode(&g); err != nil {
		t.Fatal(err)
	}
	if g.SentAt == nil || g.InvitationMessageID != "poll-1" {
		t.Fatal("poll receipt missing")
	}
}
