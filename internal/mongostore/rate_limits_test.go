package mongostore_test

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAttemptLimitSharedAcrossConcurrentInstances(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := mongostore.EnsureIndexes(ctx, db); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var allowed atomic.Int32
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := mongostore.Allow(ctx, db, "account", 10, now)
			if err != nil {
				t.Error(err)
			}
			if ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != 10 {
		t.Fatalf("allowed=%d", allowed.Load())
	}
	if ok, err := mongostore.Allow(ctx, db, "account", 10, now.Add(time.Minute)); err != nil || !ok {
		t.Fatal("new window did not reset")
	}
}
