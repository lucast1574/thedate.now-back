package core

import (
	"errors"
	"testing"
	"time"
)

func giftEvent() Event {
	now := time.Now()
	return Event{ID: "e", PaymentStatus: "paid", PublishedAt: &now, Wishlist: Wishlist{Enabled: true, Items: []Gift{{ID: "a", Title: "Cafetera", URL: "https://example.com/coffee"}, {ID: "b", Title: "Copas", URL: "https://example.com/glasses"}}}}
}
func TestGiftDecisionsPreservePromisesAndNeverMutateSnapshot(t *testing.T) {
	e := giftEvent()
	g := Guest{ID: "alice", EventID: "e"}
	now := time.Now()
	next, err := DecideGift(e, g, "a", false, now)
	if err != nil || next.Items[0].GuestID != "alice" {
		t.Fatal(next, err)
	}
	if e.Wishlist.Items[0].GuestID != "" {
		t.Fatal("snapshot mutated")
	}
	e.Wishlist = next
	if _, err = DecideGift(e, Guest{ID: "bob", EventID: "e"}, "a", false, now); !errors.Is(err, ErrGiftUnavailable) {
		t.Fatal(err)
	}
	if _, err = DecideGift(e, g, "b", false, now); !errors.Is(err, ErrGiftAssigned) {
		t.Fatal(err)
	}
	if _, err = DecideGift(e, Guest{ID: "bob", EventID: "e"}, "a", true, now); !errors.Is(err, ErrGiftUnavailable) {
		t.Fatal(err)
	}
	if _, err = DecideGift(e, g, "a", false, now); err != nil {
		t.Fatal("replay must succeed", err)
	}
	released, err := DecideGift(e, g, "a", true, now)
	if err != nil || released.Items[0].GuestID != "" {
		t.Fatal(released, err)
	}
	e.Wishlist.Items[0].Delivered = true
	if _, err = DecideGift(e, g, "a", true, now); !errors.Is(err, ErrGiftDelivered) {
		t.Fatal(err)
	}
}
func TestWishlistEligibilityAndEditsCannotForgeAssignments(t *testing.T) {
	e := giftEvent()
	g := Guest{ID: "alice", EventID: "other"}
	if _, err := DecideGift(e, g, "a", false, time.Now()); !errors.Is(err, ErrGiftDisabled) {
		t.Fatal(err)
	}
	g.EventID = e.ID
	for _, change := range []func(*Event){func(e *Event) { e.PaymentStatus = "unpaid" }, func(e *Event) { e.PublishedAt = nil }, func(e *Event) { e.IsDemo = true }} {
		copy := e
		change(&copy)
		if _, err := DecideGift(copy, g, "a", false, time.Now()); !errors.Is(err, ErrGiftDisabled) {
			t.Fatal(err)
		}
	}
	input := e.Wishlist
	input.Items = append([]Gift(nil), input.Items...)
	input.Items[0].GuestID = "forged"
	input.Items[0].Delivered = true
	edited, err := EditWishlist(e.Wishlist, input)
	if err != nil || edited.Items[0].GuestID != "" || edited.Items[0].Delivered {
		t.Fatal("forged assignment accepted", err)
	}
	e.Wishlist.Items[0].GuestID = g.ID
	input.Items = input.Items[1:]
	if _, err = EditWishlist(e.Wishlist, input); err == nil {
		t.Fatal("reserved gift removed")
	}
	input = e.Wishlist
	input.Items = append([]Gift(nil), input.Items...)
	input.Items[0].URL = "https://other.example/item"
	if _, err = EditWishlist(e.Wishlist, input); err == nil {
		t.Fatal("reserved product changed")
	}
	input = e.Wishlist
	input.Items = append([]Gift(nil), input.Items...)
	input.Items[0].URL = "javascript:alert(1)"
	if ValidWishlist(input) {
		t.Fatal("executable link allowed")
	}
}
