package core

import (
	"errors"
	"testing"
	"time"
)

func TestPartyRegistersActualSeatsAndKeepsNativeReplies(t *testing.T) {
	now := time.Now().UTC()
	e := Event{ID: "e", Capacity: 3, MaybeHoldHours: 24, PaymentStatus: "paid", PublishedAt: &now, Responses: map[string]Response{}}
	g := Guest{ID: "g", EventID: "e", Name: "Ana", Seats: 4}
	people := []Person{{Name: "Luis", LastName: "Rojas", Gender: "man"}}
	r, err := DecidePartyResponse(e, g, "going", "", &people, now)
	if err != nil || r.Seats != 2 || !r.PartyRegistered {
		t.Fatalf("party: %+v %v", r, err)
	}
	e.Responses[g.ID] = r
	// A later native WhatsApp response must retain the named party and its count.
	r, err = DecidePartyResponse(e, g, "maybe", "", nil, now)
	if err != nil || r.Seats != 2 || len(r.Companions) != 1 {
		t.Fatal("native response overwrote party")
	}
	e.Responses[g.ID] = r
	extra := Guest{ID: "other", EventID: "e", Seats: 2}
	if _, err = DecidePartyResponse(e, extra, "going", "", nil, now); !errors.Is(err, ErrCapacity) {
		t.Fatal("companions not counted in capacity")
	}
	alone := []Person{}
	r, err = DecidePartyResponse(e, g, "going", "", &alone, now)
	if err != nil || r.Seats != 1 {
		t.Fatal("could not release unused +1")
	}
	bad := []Person{{Name: "x"}}
	if _, err = DecidePartyResponse(e, g, "going", "", &bad, now); err == nil {
		t.Fatal("invalid companion accepted")
	}
	if ValidParty(people, 1) {
		t.Fatal("over invitation allowance accepted")
	}
}
func TestUnlimitedCapacityStillChecksPartyAllowance(t *testing.T) {
	now := time.Now().UTC()
	e := Event{ID: "e", CapacityUnlimited: true, PaymentStatus: "paid", PublishedAt: &now, Responses: map[string]Response{"existing": {Seats: 10000, Choice: "going"}}}
	g := Guest{ID: "g", EventID: "e", Seats: 2}
	if _, err := DecidePartyResponse(e, g, "going", "", nil, now); err != nil {
		t.Fatal(err)
	}
	people := []Person{{Name: "Ana"}, {Name: "Luis"}}
	if _, err := DecidePartyResponse(e, g, "going", "", &people, now); err == nil {
		t.Fatal("unlimited allowed too many companions")
	}
}
func TestSeatingRejectsUnknownPeopleOverbookingAndInvalidGeometry(t *testing.T) {
	guests := []Guest{{ID: "g", Name: "Ana", Seats: 2, Response: "going"}, {ID: "declined", Name: "No", Seats: 1, Response: "not_going"}}
	e := Event{Capacity: 2}
	p := DefaultSeating()
	p.Tables = []SeatingTable{{ID: "t", Name: "Mesa 1", Shape: "round", Capacity: 2, X: 10, Y: 10}}
	p.Assignments = map[string]string{"g~0": "t", "g~1": "t"}
	if err := ValidateSeating(p, e, guests); err != nil {
		t.Fatal(err)
	}
	p.Tables[0].Capacity = 1
	if ValidateSeating(p, e, guests) == nil {
		t.Fatal("table overflow")
	}
	p.Tables[0].Capacity = 3
	p.Assignments["declined~0"] = "t"
	if ValidateSeating(p, e, guests) == nil {
		t.Fatal("declined assigned")
	}
	delete(p.Assignments, "declined~0")
	p.Assignments["unknown~0"] = "t"
	if ValidateSeating(p, e, guests) == nil {
		t.Fatal("unknown assigned")
	}
	delete(p.Assignments, "unknown~0")
	e.Capacity = 1
	if ValidateSeating(p, e, guests) == nil {
		t.Fatal("aforo overflow")
	}
	e.CapacityUnlimited = true
	if ValidateSeating(p, e, guests) != nil {
		t.Fatal("unlimited rejected")
	}
	p.Tables[0].X = 1400
	if ValidateSeating(p, e, guests) == nil {
		t.Fatal("out of bounds")
	}
}
func TestResponseReconcilesPartySlots(t *testing.T) {
	p := DefaultSeating()
	p.Assignments = map[string]string{"g~0": "t", "g~1": "t", "g~2": "t", "other~0": "x"}
	next := ReconcileSeats(p, "g", Response{Seats: 1, Choice: "going"})
	if len(next) != 2 || next["g~0"] != "t" {
		t.Fatal("removed companion seat retained")
	}
	next = ReconcileSeats(p, "g", Response{Seats: 3, Choice: "not_going"})
	if len(next) != 1 || next["other~0"] != "x" {
		t.Fatal("declined seats retained or other guest lost")
	}
}
