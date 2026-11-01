package core

import (
	"testing"
	"time"
)

func TestMobileChangesExcludeHistoryAndAreDeterministic(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-time.Hour)
	later := now.Add(time.Minute)
	e := Event{ID: "event", Responses: map[string]Response{"new": {RespondedAt: later}, "old": {RespondedAt: old}}, Wishlist: Wishlist{Items: []Gift{{ID: "reserved", GuestID: "g", ReservedAt: &later}, {ID: "released", ReservedAt: &later}}}}
	changes := MobileChanges(e, now)
	again := MobileChanges(e, now)
	if len(changes) != 2 || changes[0] != again[0] || changes[1] != again[1] {
		t.Fatalf("unexpected changes: %+v", changes)
	}
	if len(MobileChanges(e, later)) != 0 {
		t.Fatal("baseline replayed")
	}
}
func TestMobileAccessRequiresCurrentIdentityAndEventAccess(t *testing.T) {
	u := User{ID: "couple", TokenVersion: 3}
	d := MobileDevice{UserID: u.ID, TokenVersion: 3}
	e := Event{OwnerID: "other", CoupleUserIDs: []string{u.ID}, PaymentStatus: "paid"}
	if !CanReceiveMobilePush(u, d, e) {
		t.Fatal("couple denied")
	}
	e.CoupleUserIDs = nil
	if CanReceiveMobilePush(u, d, e) {
		t.Fatal("revoked collaborator allowed")
	}
	e.OwnerID = u.ID
	d.TokenVersion = 2
	if CanReceiveMobilePush(u, d, e) {
		t.Fatal("revoked device allowed")
	}
	d.TokenVersion = 3
	e.IsDemo = true
	if CanReceiveMobilePush(u, d, e) {
		t.Fatal("demo allowed")
	}
}
