package core

import (
	"errors"
	"testing"
	"time"
)

func TestMaybeSettingPreservesExistingReservations(t *testing.T) {
	now := time.Now().UTC()
	expires := now.Add(time.Hour)
	disabled := false
	e := Event{ID: "e", PaymentStatus: "paid", PublishedAt: &now, Capacity: 3, MaybeHoldHours: 48, MaybeEnabled: &disabled, Responses: map[string]Response{"g": {Choice: "maybe", Seats: 2, ExpiresAt: &expires}}}
	g := Guest{ID: "g", EventID: "e", Seats: 2}
	if Occupied(e, now) != 2 {
		t.Fatal("disabling maybe released a reservation")
	}
	if _, err := DecideResponse(e, g, "maybe", "Waiting", now); !errors.Is(err, ErrMaybeDisabled) {
		t.Fatal(err)
	}
	for _, choice := range []string{"going", "not_going"} {
		if _, err := DecideResponse(e, g, choice, "", now); err != nil {
			t.Fatal(choice, err)
		}
	}
	if !e.Responses["g"].ExpiresAt.Equal(expires) || Occupied(e, expires) != 0 {
		t.Fatal("existing deadline changed")
	}
	e.MaybeEnabled = nil
	if !MaybeAllowed(e) {
		t.Fatal("legacy invitation lost maybe")
	}
}

func TestGiftSettingOverridesPublishedDesignAndStaleEditor(t *testing.T) {
	enabled, disabled := true, false
	e := Event{Celebration: &Celebration{GiftsEnabled: &disabled}, Wishlist: Wishlist{Items: []Gift{{ID: "gift"}}}, PublishedDesign: &DesignSnapshot{DesignInput: DesignInput{Celebration: &Celebration{GiftsEnabled: &enabled}}}}
	if GiftsEnabled(PublishedEvent(e)) || GiftListAvailable(e) {
		t.Fatal("published snapshot re-enabled gifts")
	}
	edited := EditedDesign(e, DesignInput{Celebration: &Celebration{GiftsEnabled: &enabled}})
	if GiftsEnabled(edited) || !*e.PublishedDesign.Celebration.GiftsEnabled {
		t.Fatal("stale editor changed the operational setting or mutated the snapshot")
	}
	e.Celebration.GiftsEnabled = &enabled
	if !GiftListAvailable(e) {
		t.Fatal("existing gifts unavailable after enabling")
	}
	e.Wishlist.Items = nil
	if GiftListAvailable(e) {
		t.Fatal("empty list advertised")
	}
}
