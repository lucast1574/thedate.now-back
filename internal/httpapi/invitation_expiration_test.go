package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
)

func TestExpireUnansweredInvitationsAfterThreeDays(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	fourDaysAgo := now.Add(-4 * 24 * time.Hour)
	twoDaysAgo := now.Add(-2 * 24 * time.Hour)

	e := core.Event{
		ID:              "event-expire",
		Kind:            "general",
		Title:           "Cumpleaños",
		Slug:            "cumple-expire",
		Capacity:        5,
		PaymentStatus:   "paid",
		PublishedAt:     &fourDaysAgo,
		ResponseVersion: 1,
		InvitationHolds: map[string]int{
			"old-unanswered": 1,
			"recent":         1,
		},
		Responses: map[string]core.Response{},
	}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}

	oldGuest := core.Guest{
		ID:          "old-unanswered",
		EventID:     e.ID,
		Name:        "Carlos",
		Phone:       "+51999111222",
		Seats:       1,
		SentAt:      &fourDaysAgo,
		Response:    "pending",
		InviteToken: strings.Repeat("a", 48),
	}
	recentGuest := core.Guest{
		ID:          "recent",
		EventID:     e.ID,
		Name:        "María",
		Phone:       "+51999333444",
		Seats:       1,
		SentAt:      &twoDaysAgo,
		Response:    "pending",
		InviteToken: strings.Repeat("b", 48),
	}
	if _, err := db.Collection("guests").InsertMany(ctx, []any{oldGuest, recentGuest}); err != nil {
		t.Fatal(err)
	}

	s := &server{db: db}
	s.expireUnansweredInvitations(ctx)

	// Check event responses and holds
	snapshot, err := (mongostore.Responses{DB: db}).Snapshot(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}

	oldResp, ok := snapshot.Responses["old-unanswered"]
	if !ok || oldResp.Choice != "not_going" || !oldResp.Finalized {
		t.Fatalf("expected old guest to be not_going finalized, got: %+v", oldResp)
	}
	if snapshot.InvitationHolds["old-unanswered"] != 0 {
		t.Fatalf("expected old guest hold to be released, got: %d", snapshot.InvitationHolds["old-unanswered"])
	}

	// Recent guest should stay untouched
	recentResp := snapshot.Responses["recent"]
	if recentResp.Choice != "" && recentResp.Choice != "pending" {
		t.Fatalf("recent guest should not be expired yet, got: %+v", recentResp)
	}
	if snapshot.InvitationHolds["recent"] != 1 {
		t.Fatalf("recent guest hold should be kept, got: %d", snapshot.InvitationHolds["recent"])
	}
}
