package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDraftPublicationPreservesLegacyAndFencesConcurrentWrites(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "draft", Kind: "general", Title: "Public", Template: "classic", AccentColor: "#112233", PaymentStatus: "paid", PublishedAt: &now, PhotoKeys: []string{"old.jpg"}}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	repo := Designs{DB: db}
	next := e
	next.Title = "Private"
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			saved, err := repo.Save(ctx, e, next, now)
			if err != nil {
				t.Error(err)
			}
			if saved {
				successes.Add(1)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent saves: %d", successes.Load())
	}
	var stored core.Event
	read := func() {
		t.Helper()
		if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&stored); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if stored.Title != "Private" || core.PublishedEvent(stored).Title != "Public" || stored.DesignVersion != 1 {
		t.Fatal("draft leaked or revision missing")
	}
	if ok, err := repo.Publish(ctx, e, now); err != nil || ok {
		t.Fatalf("stale publication: %v %v", ok, err)
	}
	if ok, err := repo.Publish(ctx, stored, now); err != nil || !ok {
		t.Fatalf("publication: %v %v", ok, err)
	}
	read()
	if core.PublishedEvent(stored).Title != "Private" || stored.PublishedDesignVersion != 1 {
		t.Fatal("publication did not advance")
	}
	if stored.Responses != nil || stored.PaymentStatus != "paid" {
		t.Fatal("non-design fields changed")
	}
	if ok, err := AttachPhoto(ctx, db, e.ID, "new.jpg", now); err != nil || !ok {
		t.Fatal(err)
	}
	read()
	if len(stored.PhotoKeys) != 2 || len(core.PublishedEvent(stored).PhotoKeys) != 1 {
		t.Fatal("uploaded draft photo leaked")
	}
}

func TestFirstPublicationFreezesDesignAndRolloutKeepsIt(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "initial", PaymentStatus: "paid", Title: "First", DesignVersion: 3}
	d := core.Deployment{EventID: e.ID, Phase: "ready"}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("deployments").InsertOne(ctx, bson.M{"_id": e.ID, "phase": "ready", "claim": "claim", "leaseUntil": now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	repo := Deployments{DB: db}
	if err := repo.Finish(ctx, d, "claim"); err != nil {
		t.Fatal(err)
	}
	var stored core.Event
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.PublishedDesignVersion != 3 || core.PublishedEvent(stored).Title != "First" {
		t.Fatal("initial snapshot missing")
	}
	next := stored
	next.Title = "Unpublished"
	if ok, err := (Designs{DB: db}).Save(ctx, stored, next, now); err != nil || !ok {
		t.Fatal(err)
	}
	if _, err := db.Collection("deployments").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"claim": "again", "leaseUntil": now.Add(time.Minute)}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Finish(ctx, d, "again"); err != nil {
		t.Fatal(err)
	}
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if core.PublishedEvent(stored).Title != "First" || stored.PublishedDesignVersion != 3 {
		t.Fatal("renderer rollout published the draft")
	}
}
