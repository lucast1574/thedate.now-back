package core

import (
	"testing"
	"time"
)

func TestReservedSeats(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	guests := []Guest{{Response: "going", Seats: 2}, {Response: "maybe", Seats: 3, MaybeExpiresAt: &future}, {Response: "maybe", Seats: 5, MaybeExpiresAt: &past}, {Response: "not_going", Seats: 7}, {Response: "pending", Seats: 4}}
	if got := ReservedSeats(guests, now); got != 5 {
		t.Fatalf("got %d reserved seats, want 5", got)
	}
}
