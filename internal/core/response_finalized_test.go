package core

import (
	"errors"
	"testing"
	"time"
)

func TestCompletedResponseRejectsFurtherChanges(t *testing.T) {
	now := time.Now()
	e := Event{ID: "e", PaymentStatus: "paid", PublishedAt: &now, Responses: map[string]Response{"g": {Choice: "going", Finalized: true, Seats: 2}}}
	g := Guest{ID: "g", EventID: "e", Seats: 2}
	for _, choice := range []string{"going", "not_going", "maybe"} {
		if _, err := DecidePartyResponse(e, g, choice, "", nil, now); !errors.Is(err, ErrResponseFinalized) {
			t.Fatalf("completed response accepted %s: %v", choice, err)
		}
	}
}

func TestMaybeRemainsOpenAndDoesNotRenewItsOriginalReservation(t *testing.T) {
	now := time.Now()
	expiry := now.Add(time.Hour)
	people := []Person{{Name: "Ana", Gender: "woman"}}
	previous := Response{Choice: "maybe", Finalized: true, PartyRegistered: true, Seats: 2, Companions: people, ExpiresAt: &expiry}
	e := Event{ID: "e", PaymentStatus: "paid", PublishedAt: &now, Capacity: 2, MaybeHoldHours: 48, Responses: map[string]Response{"g": previous}}
	g := Guest{ID: "g", EventID: e.ID, Seats: 2}
	if ResponseCompleted(previous) {
		t.Fatal("maybe incorrectly treated as terminal")
	}
	for _, choice := range []string{"going", "not_going", "maybe"} {
		next, err := DecidePartyResponse(e, g, choice, "Still waiting", nil, now)
		if err != nil {
			t.Fatal(err)
		}
		if len(next.Companions) != 1 || next.Companions[0].Name != "Ana" || next.Seats != 2 {
			t.Fatal("registered companions were lost")
		}
		if choice == "maybe" && (next.ExpiresAt == nil || !next.ExpiresAt.Equal(expiry)) {
			t.Fatal("resaving maybe renewed the reservation")
		}
	}
}
