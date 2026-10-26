package mongostore_test

import (
	"context"
	"fmt"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentGiftClaimsHaveExactlyOneWinnerAcrossReplicas(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "event", PaymentStatus: "paid", PublishedAt: &now, Wishlist: core.Wishlist{Enabled: true, Items: []core.Gift{{ID: "gift", Title: "Una cafetera", URL: "https://example.com/item"}}}}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var saved atomic.Int32
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			service := application.Wishlists{Repository: mongostore.Wishlists{DB: db}, Now: func() time.Time { return now }}
			if err := service.Choose(ctx, core.Guest{ID: fmt.Sprintf("guest-%d", index), EventID: e.ID}, "gift", false); err == nil {
				saved.Add(1)
			} else if err != core.ErrGiftUnavailable && err != application.ErrConflict {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if saved.Load() != 1 {
		t.Fatalf("got %d winners", saved.Load())
	}
	latest, err := (mongostore.Wishlists{DB: db}).Snapshot(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	owner := core.Guest{ID: latest.Wishlist.Items[0].GuestID, EventID: e.ID}
	service := application.Wishlists{Repository: mongostore.Wishlists{DB: db}, Now: func() time.Time { return now }}
	if err = service.Choose(ctx, owner, "gift", false); err != nil {
		t.Fatal("claim replay", err)
	}
	if err = service.Choose(ctx, owner, "gift", true); err != nil {
		t.Fatal("release", err)
	}
	if err = service.Choose(ctx, core.Guest{ID: "new-guest", EventID: e.ID}, "gift", false); err != nil {
		t.Fatal("reassign", err)
	}
	if ok, err := (mongostore.Wishlists{DB: db}).Commit(ctx, e.ID, latest.Wishlist.Version, latest.Wishlist, false); err != nil || ok {
		t.Fatal("stale organizer overwrote reservation", ok, err)
	}
}
func TestSameGuestCannotReserveTwoGiftsConcurrently(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now()
	e := core.Event{ID: "e", PaymentStatus: "paid", PublishedAt: &now, Wishlist: core.Wishlist{Enabled: true, Items: []core.Gift{{ID: "a", Title: "Coffee", URL: "https://example.com/a"}, {ID: "b", Title: "Glass", URL: "https://example.com/b"}}}}
	db.Collection("events").InsertOne(ctx, e)
	var wg sync.WaitGroup
	var winners atomic.Int32
	for _, id := range []string{"a", "b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			service := application.Wishlists{Repository: mongostore.Wishlists{DB: db}, Now: func() time.Time { return now }}
			if service.Choose(ctx, core.Guest{ID: "guest", EventID: e.ID}, id, false) == nil {
				winners.Add(1)
			}
		}(id)
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatal("guest got multiple gifts", winners.Load())
	}
}
