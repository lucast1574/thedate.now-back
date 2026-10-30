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
