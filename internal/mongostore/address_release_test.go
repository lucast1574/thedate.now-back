package mongostore

import (
	"context"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestReleasedAddressesKeepHistoryAndAllowOnlyOneNewReservation(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	_, err := db.Collection("events").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "slug", Value: 1}}, Options: options.Index().SetName("paid_invitation_address").SetUnique(true).SetPartialFilterExpression(bson.M{"paymentStatus": "paid"})})
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsurePaidAddressIndex(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePaidAddressIndex(ctx, db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	e := core.Event{ID: "old", Kind: "general", Slug: "cumpleronald", PaymentStatus: "paid", StartAt: now.AddDate(0, 0, -2), TimeZone: "America/Lima"}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	repo := Deployments{DB: db}
	future := e
	future.StartAt = now.AddDate(0, 0, 2)
	if err := repo.ReleaseAddress(ctx, future, now); err == nil {
		t.Fatal("future event released")
	}
	if err := repo.ReleaseAddress(ctx, e, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReleaseAddress(ctx, e, now); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"new", "duplicate"} {
		_, err := db.Collection("events").InsertOne(ctx, bson.M{"_id": id, "kind": "general", "slug": e.Slug, "paymentStatus": "paid"})
		if i == 0 && err != nil {
			t.Fatalf("released address still reserved: %v", err)
		}
		if i == 1 && !mongo.IsDuplicateKeyError(err) {
			t.Fatalf("two live reservations allowed: %v", err)
		}
	}
	var old core.Event
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&old); err != nil {
		t.Fatal(err)
	}
	if old.Slug != e.Slug || old.AddressReleasedAt == nil || old.PaymentStatus != "paid" {
		t.Fatal("history changed")
	}
}
