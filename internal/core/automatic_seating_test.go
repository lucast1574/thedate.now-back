package core

import (
	"fmt"
	"testing"
)

func TestAutomaticSeatingConfirmationCategoryAndParty(t *testing.T) {
	e := Event{Capacity: 100, Responses: map[string]Response{}}
	guests := []Guest{{ID: "waiting", Seats: 2, Response: "pending", Family: "Familia del novio"}}
	confirm := func(id, category string, seats int) {
		t.Helper()
		guests = append(guests, Guest{ID: id, Seats: 2, Family: category, Response: "pending"})
		r := Response{Choice: "going", Seats: seats, PartyRegistered: true}
		if seats == 2 {
			r.Companions = []Person{{Name: "Pareja"}}
		}
		p := SeatResponse(e, guests, id, r)
		e.Seating = &p
		e.Responses[id] = r
		if err := ValidateSeating(p, e, guests); err != nil {
			t.Fatal(err)
		}
	}
	confirm("a", "Familia del novio", 2)
	if len(e.Seating.Tables) != 1 || e.Seating.Tables[0].Capacity != 10 || len(e.Seating.Assignments) != 2 {
		t.Fatal("default capacity or party count")
	}
	for n := 0; n < 7; n++ {
		confirm(fmt.Sprint(n), "Familia del novio", 1)
	}
	confirm("pair", "Familia del novio", 2)
	if len(e.Seating.Tables) != 2 || e.Seating.Assignments["pair~0"] != e.Seating.Assignments["pair~1"] || e.Seating.Assignments["pair~0"] == e.Seating.Assignments["a~0"] {
		t.Fatal("party split into last single seat")
	}
	confirm("bride", "Familia de la novia", 1)
	if len(e.Seating.Tables) != 3 {
		t.Fatal("unrelated category mixed")
	}
	e.Seating.Tables[0].Capacity = 12
	confirm("more", "Familia del novio", 2)
	if e.Seating.Assignments["more~0"] != e.Seating.Tables[0].ID {
		t.Fatal("manual capacity ignored")
	}
	before := e.Seating.Assignments["bride~0"]
	p := SeatResponse(e, guests, "a", Response{Choice: "not_going", Seats: 2})
	if p.Assignments["a~0"] != "" || p.Assignments["a~1"] != "" || p.Assignments["bride~0"] != before {
		t.Fatal("decline did not release only its seats")
	}
	p = SeatResponse(e, guests, "a", Response{Choice: "maybe", Seats: 2})
	if p.Assignments["a~0"] != "" {
		t.Fatal("maybe retained seats")
	}
	if e.Responses["a"].Choice != "going" || e.Seating.Assignments["a~0"] == "" {
		t.Fatal("input snapshot mutated")
	}
}

func TestSeatingRejectsPendingAndSplitPartiesAllowsManyManualTables(t *testing.T) {
	e := Event{Capacity: 20}
	guests := []Guest{{ID: "a", Seats: 2, Response: "going", Companions: []Person{{Name: "Pareja"}}}, {ID: "pending", Seats: 1, Response: "pending"}}
	p := DefaultSeating()
	for n := 0; n < 101; n++ {
		p.Tables = append(p.Tables, SeatingTable{ID: fmt.Sprint(n), Name: fmt.Sprintf("Mesa %d", n), Shape: "round", Capacity: 10})
	}
	if err := ValidateSeating(p, e, guests); err != nil {
		t.Fatal(err)
	}
	p.Assignments["pending~0"] = "0"
	if ValidateSeating(p, e, guests) == nil {
		t.Fatal("pending seated")
	}
	delete(p.Assignments, "pending~0")
	p.Assignments["a~0"], p.Assignments["a~1"] = "0", "1"
	if ValidateSeating(p, e, guests) == nil {
		t.Fatal("split pair allowed")
	}
	p.Assignments["a~1"] = "0"
	if err := ValidateSeating(p, e, guests); err != nil {
		t.Fatal(err)
	}
}

func TestPossibleCompanionGetsNoSeatUntilRegistered(t *testing.T) {
	g := Guest{ID: "a", Seats: 2, Response: "pending"}
	e := Event{Capacity: 10, Responses: map[string]Response{}}
	r := Response{Choice: "going", Seats: 2}
	p := SeatResponse(e, []Guest{g}, g.ID, r)
	if len(p.Assignments) != 1 || p.Assignments["a~1"] != "" {
		t.Fatal("possible +1 seated")
	}
	e.Seating, e.Responses[g.ID] = &p, r
	r.Companions, r.PartyRegistered = []Person{{Name: "Pareja"}}, true
	registered := SeatResponse(e, []Guest{g}, g.ID, r)
	if len(registered.Assignments) != 2 || registered.Assignments["a~1"] != p.Assignments["a~0"] {
		t.Fatal("registered companion did not join titular")
	}
}
