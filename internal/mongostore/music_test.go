package mongostore

import (
	"context"
	"fmt"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAudioUploadsRespectAtomicLimit(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	if _, err := db.Collection("events").InsertOne(ctx, core.Event{ID: "event"}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for n := 0; n < 40; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ok, err := AttachAudio(ctx, db, "event", fmt.Sprintf("audio-%d.mp3", n), time.Now())
			if err != nil {
				t.Error(err)
			}
			if ok {
				accepted.Add(1)
			}
		}(n)
	}
	wg.Wait()
	var event core.Event
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": "event"}).Decode(&event); err != nil {
		t.Fatal(err)
	}
	if accepted.Load() != 20 || len(event.AudioKeys) != 20 {
		t.Fatalf("accepted %d; saved %d", accepted.Load(), len(event.AudioKeys))
	}
}
