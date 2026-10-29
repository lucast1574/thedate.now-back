package core

import (
	"reflect"
	"testing"
	"time"
)

func TestReimportOnlyChangesDescriptionsAndFlagsProtectedChanges(t *testing.T) {
	now := time.Now()
	old := Guest{ID: "legacy", EventID: "event", Name: "Ana", LastName: "Saved", Family: "Amigos", Gender: "woman", Phone: "+51925431535", Seats: 2, InviteToken: "keep", Response: "going", QueuePosition: 7, SentAt: &now, Companions: []Person{{Name: "Luis"}}}
	next, review := MergeGuestImport(old, Guest{Name: " Ana updated ", Family: "Familia de la novia", Seats: 1})
	want := old
	want.Name = "Ana updated"
	want.Family = "Familia de la novia"
	if !reflect.DeepEqual(next, want) || !review {
		t.Fatalf("destructive reimport: %+v review=%v", next, review)
	}
	next, review = MergeGuestImport(old, Guest{Name: old.Name, Seats: 2})
	if !reflect.DeepEqual(next, old) || review {
		t.Fatal("empty optional fields changed saved data")
	}
	if GuestPhoneID(old.EventID, old.Phone) != GuestPhoneID(old.EventID, old.Phone) || GuestPhoneID("other", old.Phone) == GuestPhoneID(old.EventID, old.Phone) {
		t.Fatal("wrong phone identity scope")
	}
}
