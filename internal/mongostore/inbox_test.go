package mongostore_test

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"sync"
	"testing"
	"time"
)

func TestInboxReplayAndAttendedWatermark(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	repo := mongostore.Inbox{DB: db}
	now := time.Now().UTC().Truncate(time.Millisecond)
	if err := mongostore.EnsureInboxIndexes(ctx, db); err != nil {
		t.Fatal(err)
	}
	first := core.InboxMessage{ID: "first", EventID: "event", GuestID: "guest", Direction: "in", ReceivedAt: now, CreatedAt: now}
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if _, err := repo.Put(ctx, first); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	list, err := repo.Messages(ctx, "event", "guest", "")
	if err != nil || len(list) != 1 {
		t.Fatal("duplicate message", err)
	}
	attended := true
	if err = repo.Mark(ctx, "event", "guest", first.ID, &attended); err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = "second"
	// MongoDB truncates timestamps to milliseconds: a second message can have
	// exactly the same timestamp and must still reopen the conversation.
	second.ReceivedAt = now
	repo.Put(ctx, second)
	if err = repo.Mark(ctx, "event", "guest", first.ID, &attended); err != nil {
		t.Fatal(err)
	}
	conversations, err := repo.Conversations(ctx, "event")
	if err != nil || len(conversations) != 1 || !conversations[0].Pending || !conversations[0].Unread {
		t.Fatal("new incoming message was hidden by an old attendance update", err)
	}
	if err = repo.Mark(ctx, "foreign-event", "guest", first.ID, nil); err == nil {
		t.Fatal("foreign message accepted")
	}
}
