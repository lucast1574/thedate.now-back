package core

import (
	"errors"
	"testing"
	"time"
)

func TestResponseDecisionsRespectExpiryAndOwnSeats(t *testing.T) {
	now := time.Now().UTC()
	expired := now.Add(-time.Second)
	e := Event{ID: "e", Capacity: 2, MaybeHoldHours: 24, PaymentStatus: "paid", PublishedAt: &now, Responses: map[string]Response{"old": {Seats: 2, Choice: "maybe", ExpiresAt: &expired}}}
	g := Guest{ID: "g", EventID: "e", Seats: 2}
	r, err := DecideResponse(e, g, "maybe", "reason", now)
	if err != nil || r.ExpiresAt == nil || !r.ExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Fatal("expired reservation blocked new guest")
	}
	e.Responses[g.ID] = r
	if _, err = DecideResponse(e, g, "going", "", now); err != nil {
		t.Fatal("changing own response charged seats twice")
	}
	if _, err = DecideResponse(e, Guest{ID: "other", EventID: "e", Seats: 1}, "going", "", now); !errors.Is(err, ErrCapacity) {
		t.Fatal("overbooking accepted")
	}
	e.PublishedAt = nil
	if _, err = DecideResponse(e, g, "not_going", "", now); !errors.Is(err, ErrUnavailable) {
		t.Fatal("unpublished RSVP accepted")
	}
}
func TestProductsKeepWeddingPolicySeparate(t *testing.T) {
	wedding, err := ProductFor("wedding")
	if err != nil {
		t.Fatal(err)
	}
	general, err := ProductFor("general")
	if err != nil {
		t.Fatal(err)
	}
	if wedding.AllowVirtual || !wedding.AllowCouples || wedding.Role != "planner" || wedding.PriceCents != 2500 {
		t.Fatal("wedding policy")
	}
	if !general.AllowVirtual || general.AllowCouples || general.Role != "organizer" || general.PriceCents != 500 {
		t.Fatal("general policy")
	}
	if CanCreateEvent("wedding", "organizer") || CanCreateEvent("general", "planner") || CanCreateEvent("unknown", "admin") {
		t.Fatal("cross-product role accepted")
	}
}
func TestCoupleAccessRequiresVerifiedEmailOwnership(t *testing.T) {
	now := time.Now().UTC()
	e := Event{ID: "e", Kind: "wedding", PaymentStatus: "paid", CoupleInvites: map[string]CoupleInvite{"hash": {Email: "couple@gmail.com", TokenHash: "hash", ExpiresAt: now.Add(time.Hour)}}}
	u := User{ID: "couple", Email: "couple@gmail.com", Role: "organizer", GoogleSub: "google", IdentityPolicy: 1}
	if _, _, err := AcceptCoupleInvite(e, "hash", u, now); !errors.Is(err, ErrCoupleAccess) {
		t.Fatal("non-authoritative email accepted")
	}
	u.GoogleAuthoritative = true
	ids, invites, err := AcceptCoupleInvite(e, "hash", u, now)
	if err != nil || len(ids) != 1 || len(invites) != 0 {
		t.Fatal("verified Google account could not join")
	}
	if _, err = AttachCouple(Event{Kind: "general", PaymentStatus: "paid"}, "couple", now); err == nil {
		t.Fatal("general event accepted couple access")
	}
}

func TestExplicitPortalAccessDoesNotExpandCoupleRole(t *testing.T) {
	if !UserCanCreate("wedding", User{Role: "organizer", Portals: []string{"wedding"}}) {
		t.Fatal("explicit second portal ignored")
	}
	if UserCanCreate("general", User{Role: "couple", Portals: []string{"general"}}) {
		t.Fatal("couple role gained creator access")
	}
}
