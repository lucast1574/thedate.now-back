package mongostore_test

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDeploymentClaimExclusiveAndPublicationRecoversAfterRestart(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	e := core.Event{ID: "event", Kind: "general", Slug: "party", PaymentStatus: "paid"}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	repo := mongostore.Deployments{DB: db}
	d, err := repo.Queue(ctx, e, "image")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var claimed atomic.Int32
	var owner string
	var mu sync.Mutex
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, claim, err := repo.Claim(ctx)
			if err == nil {
				claimed.Add(1)
				mu.Lock()
				owner = claim
				mu.Unlock()
			} else if err != mongo.ErrNoDocuments {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if claimed.Load() != 1 {
		t.Fatal("multiple workers acquired the invitation")
	}
	d.ProjectID = "project"
	d.ApplicationID = "application"
	d.Phase = "ready"
	if err = repo.Save(ctx, d, "wrong-owner"); err == nil {
		t.Fatal("foreign worker changed checkpoint")
	}
	if err = repo.Save(ctx, d, owner); err != nil {
		t.Fatal(err)
	}
	// Process dies after checkpoint but before marking the event published.
	_, err = db.Collection("deployments").UpdateByID(ctx, d.EventID, bson.M{"$set": bson.M{"leaseUntil": time.Now().Add(-time.Minute)}})
	if err != nil {
		t.Fatal(err)
	}
	restarted := mongostore.Deployments{DB: db}
	d, claim, err := restarted.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Phase != "ready" || d.ApplicationID != "application" {
		t.Fatal("checkpoint lost")
	}
	if err = restarted.Finish(ctx, d, claim); err != nil {
		t.Fatal(err)
	}
	if err = db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&e); err != nil {
		t.Fatal(err)
	}
	if e.PublishedAt == nil {
		t.Fatal("ready job left event unpublished")
	}
	if _, _, err = restarted.Claim(ctx); err != mongo.ErrNoDocuments {
		t.Fatal("finished job was claimed again")
	}
}
