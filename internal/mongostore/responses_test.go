package mongostore_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentReservationsAcrossServices(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "event", Kind: "general", Capacity: 7, MaybeHoldHours: 48, PaymentStatus: "paid", PublishedAt: &now}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var saved atomic.Int32
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := application.Responses{Repository: mongostore.Responses{DB: db}, Now: func() time.Time { return now }}
			err := s.Apply(ctx, e.ID, core.Guest{ID: fmt.Sprintf("guest-%d", i), EventID: e.ID, Seats: 1}, "going", "", "")
			if err == nil {
				saved.Add(1)
			} else if !errors.Is(err, core.ErrCapacity) && !errors.Is(err, application.ErrConflict) {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	latest, err := (mongostore.Responses{DB: db}).Snapshot(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Load() != 7 || core.Occupied(latest, now) != 7 {
		t.Fatalf("saved=%d occupied=%d", saved.Load(), core.Occupied(latest, now))
	}
}
func TestReceiptAndResponseCommitTogether(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "event", Capacity: 1, MaybeHoldHours: 48, PaymentStatus: "paid", PublishedAt: &now}
	_, err := db.Collection("events").InsertOne(ctx, e)
	if err != nil {
		t.Fatal(err)
	}
	g := core.Guest{ID: "guest", EventID: e.ID, Seats: 1}
	repo := mongostore.Responses{DB: db}
	s := application.Responses{Repository: repo, Now: func() time.Time { return now }}
	if err = s.Apply(ctx, e.ID, g, "maybe", "", "receipt-a"); err != nil {
		t.Fatal(err)
	}
	restarted := application.Responses{Repository: mongostore.Responses{DB: db}, Now: s.Now}
	if err = restarted.Apply(ctx, e.ID, g, "not_going", "", "receipt-b"); err != nil {
		t.Fatal(err)
	}
	if err = restarted.Apply(ctx, e.ID, g, "maybe", "", "receipt-a"); !errors.Is(err, core.ErrDuplicate) {
		t.Fatalf("replay: %v", err)
	}
	latest, err := repo.Snapshot(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Responses[g.ID].Choice != "not_going" || core.Occupied(latest, now) != 0 {
		t.Fatal("replay overwrote a later response")
	}
}
func TestCapacityChangeRacesWithRSVP(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := db.Collection("events").InsertOne(ctx, core.Event{ID: "event", Capacity: 20, MaybeHoldHours: 48, PaymentStatus: "paid", PublishedAt: &now})
	if err != nil {
		t.Fatal(err)
	}
	s := application.Responses{Repository: mongostore.Responses{DB: db}, Now: func() time.Time { return now }}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := s.Apply(ctx, "event", core.Guest{ID: fmt.Sprintf("g-%d", i), EventID: "event", Seats: 1}, "going", "", "")
			if err != nil && !errors.Is(err, core.ErrCapacity) && !errors.Is(err, application.ErrConflict) {
				t.Error(err)
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.UpdateEvent(ctx, "event", validInput(1))
		if err != nil && !errors.Is(err, core.ErrCapacity) && !errors.Is(err, application.ErrConflict) {
			t.Error(err)
		}
	}()
	wg.Wait()
	var latest core.Event
	if err = db.Collection("events").FindOne(ctx, bson.M{"_id": "event"}).Decode(&latest); err != nil {
		t.Fatal(err)
	}
	if core.Occupied(latest, now) > latest.Capacity {
		t.Fatal("capacity invariant broken")
	}
	if latest.Capacity > 1 {
		if err = s.UpdateEvent(ctx, "event", validInput(1)); !errors.Is(err, core.ErrCapacity) {
			t.Fatalf("shrink below reservations: %v", err)
		}
	}
}
func TestLegacyResponsesMigrateWithoutReservingExpiredMaybe(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	expired := now.Add(-time.Hour)
	_, err := db.Collection("events").InsertOne(ctx, core.Event{ID: "event", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("guests").InsertOne(ctx, core.Guest{ID: "legacy", EventID: "event", Seats: 2, Response: "maybe", MaybeExpiresAt: &expired})
	if err != nil {
		t.Fatal(err)
	}
	e, err := (mongostore.Responses{DB: db}).Snapshot(ctx, "event")
	if err != nil {
		t.Fatal(err)
	}
	if e.ResponseVersion != 1 || len(e.Responses) != 1 || core.Occupied(e, now) != 0 {
		t.Fatal("invalid legacy migration")
	}
}

func validInput(capacity int) core.EventInput {
	return core.EventInput{Kind: "general", Title: "updated", StartAt: time.Now().UTC(), TimeZone: "UTC", Organizer: "Host", IsVirtual: true, VirtualURL: "https://meet.google.com/example", Capacity: capacity, MaybeHoldHours: 48, Template: "classic", AccentColor: "#abcdef"}
}
