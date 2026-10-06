package mongostore_test

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"testing"
	"time"
)

func TestRendererRolloutKeepsResourcesAndOriginalPublication(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	published := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Millisecond)
	e := core.Event{ID: "event", Kind: "general", Slug: "our-party", PaymentStatus: "paid", PublishedAt: &published}
	db.Collection("events").InsertOne(ctx, e)
	repo := mongostore.Deployments{DB: db}
	old := core.Deployment{EventID: e.ID, ProjectID: "project", ApplicationID: "app", DomainID: "domain", Host: "our-party.thedate.now", Phase: "ready", Image: "old"}
	db.Collection("deployments").InsertOne(ctx, bson.M{"_id": old.EventID, "projectId": old.ProjectID, "applicationId": old.ApplicationID, "domainId": old.DomainID, "host": old.Host, "phase": "ready", "finalized": true, "image": "old"})
	if err := repo.RollRenderer(ctx, "new"); err != nil {
		t.Fatal(err)
	}
	if repo.RendererImage(ctx, "fallback") != "new" {
		t.Fatal("new publications did not adopt release")
	}
	d, claim, err := repo.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Image != "new" || d.ApplicationID != old.ApplicationID || d.ProjectID != old.ProjectID || d.Host != old.Host || d.DomainID != old.DomainID {
		t.Fatal("rollout lost existing resource checkpoints")
	}
	d.Phase = "ready"
	if err = repo.Save(ctx, d, claim); err != nil {
		t.Fatal(err)
	}
	if err = repo.Finish(ctx, d, claim); err != nil {
		t.Fatal(err)
	}
	var saved core.Event
	db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved)
	if saved.PublishedAt == nil || !saved.PublishedAt.Equal(published) {
		t.Fatal("rollout replaced original publication")
	}
	// A later release must not steal an in-flight lease.
	db.Collection("deployments").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"claim": "active", "leaseUntil": time.Now().Add(time.Hour)}})
	repo.RollRenderer(ctx, "newer")
	d, _ = repo.Get(ctx, e.ID)
	if d.Image != "new" {
		t.Fatal("rollout stole active lease")
	}
}
