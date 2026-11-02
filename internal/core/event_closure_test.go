package core

import (
	"testing"
	"time"
)

func TestEventClosesAtFollowingLocalMidnight(t *testing.T) {
	for _, row := range []struct{ zone, start, end string }{
		{"America/Bogota", "2026-11-01T01:00:00Z", "2026-11-01T05:00:00Z"},
		{"Pacific/Kiritimati", "2026-11-01T01:00:00Z", "2026-11-01T10:00:00Z"},
		{"America/New_York", "2026-11-01T05:30:00Z", "2026-11-02T05:00:00Z"},
		{"America/New_York", "2026-03-08T06:30:00Z", "2026-03-09T04:00:00Z"},
		{"invalid", "2026-11-01T01:00:00Z", "2026-11-02T00:00:00Z"},
	} {
		start, _ := time.Parse(time.RFC3339, row.start)
		end, _ := time.Parse(time.RFC3339, row.end)
		for _, kind := range []string{"general", "wedding"} {
			e := Event{Kind: kind, StartAt: start, TimeZone: row.zone}
			if !EventClosingAt(e).Equal(end) || EventClosed(e, end.Add(-time.Nanosecond)) || !EventClosed(e, end) {
				t.Fatalf("%s/%s: incorrect midnight %v", kind, row.zone, EventClosingAt(e))
			}
			e.IsDemo = true
			if EventClosed(e, end) {
				t.Fatal("demo expired")
			}
		}
	}
}

func TestClosedEventCannotOfferSeatsAlreadyReserved(t *testing.T) {
	now := time.Now().UTC()
	e := Event{ID: "event", StartAt: now.AddDate(0, 0, -2), PaymentStatus: "paid", PublishedAt: &now, InvitationHolds: map[string]int{"guest": 1}}
	if CanOfferInvitation(e, Guest{ID: "guest", EventID: e.ID, Seats: 1}, now) {
		t.Fatal("closed event offered seats")
	}
}
