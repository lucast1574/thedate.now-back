package mongostore_test

import (
	"context"
	"errors"
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

func TestCoupleInvitationsShareAtomicQuotaWithAccounts(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := db.Collection("events").InsertOne(ctx, core.Event{ID: "wedding", Kind: "wedding", PaymentStatus: "paid", CoupleUserIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	repo := mongostore.Couples{DB: db}
	s := application.Couples{Repository: repo, Now: func() time.Time { return now }}
	var wg sync.WaitGroup
	var saved atomic.Int32
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i%2 == 0 {
				err = s.Attach(ctx, "wedding", fmt.Sprintf("user-%d", i))
			} else {
				err = s.Invite(ctx, "wedding", core.CoupleInvite{Email: fmt.Sprintf("couple%d@gmail.com", i), TokenHash: fmt.Sprintf("hash%d", i), ExpiresAt: now.Add(time.Hour)})
			}
			if err == nil {
				saved.Add(1)
			} else if !errors.Is(err, core.ErrCoupleCapacity) && !errors.Is(err, application.ErrConflict) {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	e, err := repo.Snapshot(ctx, "wedding")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Load() != 2 || len(e.CoupleUserIDs)+len(core.ActiveCoupleInvites(e, now)) != 2 {
		t.Fatal("couple quota broken")
	}
}
