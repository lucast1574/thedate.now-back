package core

import (
	"go.mongodb.org/mongo-driver/v2/bson"
	"math"
	"testing"
)

func TestVenueAreasStayInsidePlanAndRoundTrip(t *testing.T) {
	p := DefaultSeating()
	p.Areas = []VenueArea{{ID: "stage", Name: "Escenario", Kind: "stage", X: 590, Y: 100, Width: 420, Height: 140}}
	if err := ValidateSeating(p, Event{Capacity: 50}, nil); err != nil {
		t.Fatal(err)
	}
	data, err := bson.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var saved SeatingPlan
	if err := bson.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Areas) != 1 || saved.Areas[0] != p.Areas[0] {
		t.Fatal("venue areas lost on storage")
	}
	for _, kind := range []string{"stage", "bar", "snacks", "dance", "lounge", "photo", "entrance"} {
		p.Areas[0].Kind = kind
		if err := ValidateSeating(p, Event{Capacity: 50}, nil); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
}

func TestVenueAreasRejectInvalidGeometryAndIdentity(t *testing.T) {
	base := VenueArea{ID: "stage", Name: "Escenario", Kind: "stage", X: 590, Y: 100, Width: 420, Height: 140}
	for _, change := range []func(*VenueArea){
		func(a *VenueArea) { a.X = 1500 }, func(a *VenueArea) { a.Y = -1 }, func(a *VenueArea) { a.Width = math.NaN() },
		func(a *VenueArea) { a.Height = 601 }, func(a *VenueArea) { a.Kind = "foreign" }, func(a *VenueArea) { a.Name = " " },
	} {
		p := DefaultSeating()
		a := base
		change(&a)
		p.Areas = []VenueArea{a}
		if ValidateSeating(p, Event{Capacity: 50}, nil) == nil {
			t.Fatal("invalid area accepted")
		}
	}
	p := DefaultSeating()
	p.Areas = []VenueArea{base, base}
	if ValidateVenueAreas(p) == nil {
		t.Fatal("duplicate IDs accepted")
	}
	p.Areas = []VenueArea{base}
	p.Tables = []SeatingTable{{ID: base.ID}}
	if ValidateVenueAreas(p) == nil {
		t.Fatal("table and area share an ID")
	}
	p.Tables = nil
	p.Areas = make([]VenueArea, 31)
	if ValidateVenueAreas(p) == nil {
		t.Fatal("too many areas accepted")
	}
}
