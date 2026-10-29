package mongostore_test

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"testing"
	"time"
)

func TestAddressChangesBeforePublicationAndIsFencedAgainstPublishing(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "address", Kind: "general", Slug: "old-address", PaymentStatus: "paid", ResponseVersion: 1, Responses: map[string]core.Response{}}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	repo := mongostore.Responses{DB: db}
	s := application.Responses{Repository: repo, Now: func() time.Time { return now }}
	in := validInput(20)
	in.Slug = "new-address"
	if err := s.UpdateEvent(ctx, e.ID, in); err != nil {
		t.Fatal(err)
	}
	current, err := repo.Snapshot(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Slug != "new-address" {
		t.Fatal("address was silently ignored")
	}
	if _, err := db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"publishedAt": now}}); err != nil {
		t.Fatal(err)
	}
	in.Slug = "changed-too-late"
	if saved, err := repo.CommitEvent(ctx, current, in, now); err != nil || saved {
		t.Fatal("stale edit changed a published address")
	}
	if err := s.UpdateEvent(ctx, e.ID, in); !errors.Is(err, core.ErrPublishedAddress) {
		t.Fatalf("published address: %v", err)
	}
	in.Slug = "new-address"
	in.Title = "Cumpleaños de Javier"
	if err := s.UpdateEvent(ctx, e.ID, in); err != nil {
		t.Fatal("normal metadata update blocked", err)
	}
	in.Slug = "save"
	if err := s.UpdateEvent(ctx, e.ID, in); err == nil {
		t.Fatal("reserved domain accepted on update")
	}
}
