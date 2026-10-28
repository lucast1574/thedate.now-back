package core

import (
	"testing"
	"time"
)

func TestInvitationOffersShareCapacityWithResponses(t *testing.T) {
	now := time.Now().UTC()
	e := Event{ID: "event", PaymentStatus: "paid", PublishedAt: &now, Capacity: 3, InvitationHolds: map[string]int{"first": 2}, Responses: map[string]Response{"confirmed": {Seats: 1, Choice: "going"}}}
	next := Guest{ID: "next", EventID: e.ID, Seats: 1}
	if CanOfferInvitation(e, next, now) {
		t.Fatal("offered seats were ignored")
	}
	first := Guest{ID: "first", EventID: e.ID, Seats: 2}
	if _, err := DecideResponse(e, first, "going", "", now); err != nil {
		t.Fatalf("guest could not accept own offered seats: %v", err)
	}
	delete(e.InvitationHolds, "first")
	if !CanOfferInvitation(e, next, now) {
		t.Fatal("released seats did not admit next invitation")
	}
	next.Seats = 3
	if CanOfferInvitation(e, next, now) {
		t.Fatal("household was partially admitted")
	}
}

func TestUnfinishedSendBecomesUncertainAfterTwoMinutes(t *testing.T) {
	now := time.Now().UTC()
	started := now.Add(-3 * time.Minute)
	g := Guest{SendState: "sending", SendStartedAt: &started}
	if InvitationSendState(g, now) != "unknown" {
		t.Fatal("stalled send was not surfaced for review")
	}
	started = now
	if InvitationSendState(g, now) != "sending" {
		t.Fatal("active send was incorrectly marked uncertain")
	}
}
