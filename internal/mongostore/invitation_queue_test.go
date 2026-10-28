package mongostore

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
)

func TestConcurrentInvitationOffersDoNotOverbookAcrossReplicas(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "queue", PaymentStatus: "paid", PublishedAt: &now, Capacity: 4, ResponseVersion: 1, Responses: map[string]core.Response{}}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			store := InvitationQueue{DB: db}
			ok, err := store.Reserve(ctx, e.ID, core.Guest{ID: fmt.Sprintf("g%d", index), EventID: e.ID, Seats: 1}, now)
			if err != nil {
				t.Error(err)
			}
			if ok {
				accepted.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if accepted.Load() != 4 {
		t.Fatalf("offered %d seats for capacity 4", accepted.Load())
	}
	latest, err := (Responses{DB: db}).Snapshot(ctx, e.ID)
	if err != nil || core.Occupied(latest, now) != 4 {
		t.Fatalf("invalid canonical capacity: %+v %v", latest.InvitationHolds, err)
	}
}
