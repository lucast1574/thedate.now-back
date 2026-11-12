package mongostore_test

import (
	"context"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestDisablingGiftsRejectsStaleClaimsAndReservations(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "e", PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(time.Hour), ResponseVersion: 1, Wishlist: core.Wishlist{Version: 1, Items: []core.Gift{{ID: "gift"}}}, Responses: map[string]core.Response{"g": {Choice: "going"}}, GiftFollowups: map[string]string{"g": "pending"}}
	g := core.Guest{ID: "g", EventID: "e", Seats: 1, InviteToken: "token"}
	_, _ = db.Collection("events").InsertOne(ctx, e)
	_, _ = db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"celebration.giftsEnabled": false}})
	if ok, err := (mongostore.Responses{DB: db}).ClaimGiftFollowup(ctx, e, g, now); err != nil || ok {
		t.Fatal("stale worker claimed disabled gifts", ok, err)
	}
	if ok, err := (mongostore.Wishlists{DB: db}).Commit(ctx, e.ID, 1, e.Wishlist, true); err != nil || ok {
		t.Fatal("stale guest reserved a disabled gift", ok, err)
	}
}

func TestMaybeToggleInvalidatesAnInFlightResponse(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now, disabled := time.Now().UTC(), false
	e := core.Event{ID: "e", Slug: "event", PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(time.Hour), Capacity: 5, ResponseVersion: 1}
	_, _ = db.Collection("events").InsertOne(ctx, e)
	repo := mongostore.Responses{DB: db}
	in := core.EventInput{Slug: e.Slug, StartAt: e.StartAt, Capacity: e.Capacity, MaybeEnabled: &disabled, MaybeHoldHours: 48}
	if ok, err := repo.CommitEvent(ctx, e, in, now); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := repo.CommitResponse(ctx, e, "g", core.Response{Choice: "maybe", Seats: 1}, ""); err != nil || ok {
		t.Fatal("stale response ignored the option update", ok, err)
	}
	latest, err := repo.Snapshot(ctx, e.ID)
	if err != nil || core.MaybeAllowed(latest) {
		t.Fatal("setting was not persisted", err)
	}
}
