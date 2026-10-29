package mongostore_test

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentManualAndExcelCreatesOneGuest(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	store := mongostore.GuestImports{DB: db}
	var added atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			guest := core.Guest{ID: core.GuestPhoneID("event", "+51925431535"), EventID: "event", Phone: "+51925431535", Name: "Ana", Seats: 2, InviteToken: time.Now().String()}
			if i%2 == 0 {
				yes, err := store.Add(ctx, guest)
				if err != nil {
					t.Error(err)
				}
				if yes {
					added.Add(1)
				}
			} else {
				result, err := store.Apply(ctx, guest)
				if err != nil {
					t.Error(err)
				}
				if result.Added {
					added.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()
	count, err := db.Collection("guests").CountDocuments(ctx, bson.M{"eventId": "event"})
	if err != nil || count != 1 || added.Load() != 1 {
		t.Fatalf("duplicates: count=%d added=%d err=%v", count, added.Load(), err)
	}
}

func TestExcelUpdatesLegacyMetadataWithoutChangingRSVPOrQueue(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	store := mongostore.GuestImports{DB: db}
	now := time.Now().UTC().Truncate(time.Millisecond)
	old := core.Guest{ID: "legacy-uuid", EventID: "event", Phone: "+51925431535", Name: "Ana", LastName: "Original", Family: "Amigos", Seats: 2, InviteToken: "stable-token", Response: "going", QueuePosition: 4, SentAt: &now, Companions: []core.Person{{Name: "Luis"}}, PartyRegistered: true}
	if _, err := db.Collection("guests").InsertOne(ctx, old); err != nil {
		t.Fatal(err)
	}
	// PartyRegistered is a derived view field; compare the actual persisted record.
	var persisted core.Guest
	if err := db.Collection("guests").FindOne(ctx, bson.M{"_id": old.ID}).Decode(&persisted); err != nil {
		t.Fatal(err)
	}
	old = persisted
	incoming := core.Guest{ID: core.GuestPhoneID(old.EventID, old.Phone), EventID: old.EventID, Phone: old.Phone, Name: "Ana corrected", Family: "Familia de la novia", Seats: 1, InviteToken: "never-replace"}
	result, err := store.Apply(ctx, incoming)
	if err != nil || !result.Updated || result.Added || !result.Review {
		t.Fatalf("bad report %+v %v", result, err)
	}
	var saved core.Guest
	if err = db.Collection("guests").FindOne(ctx, bson.M{"_id": old.ID}).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	want := old
	want.Name = incoming.Name
	want.Family = incoming.Family
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("existing invitation changed: %+v", saved)
	}
	result, err = store.Apply(ctx, incoming)
	if err != nil || result.Added || result.Updated || !result.Review {
		t.Fatalf("retry changed data %+v %v", result, err)
	}
	count, _ := db.Collection("guests").CountDocuments(ctx, bson.M{"eventId": "event"})
	if count != 1 {
		t.Fatal("legacy guest duplicated")
	}
}
