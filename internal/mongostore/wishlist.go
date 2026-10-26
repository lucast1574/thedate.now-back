package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

type Wishlists struct{ DB *mongo.Database }

func (s Wishlists) Snapshot(ctx context.Context, id string) (core.Event, error) {
	var e core.Event
	err := s.DB.Collection("events").FindOne(ctx, bson.M{"_id": id}).Decode(&e)
	return e, err
}
func (s Wishlists) Commit(ctx context.Context, id string, version int64, next core.Wishlist, public bool) (bool, error) {
	filter := bson.M{"_id": id, "wishlist.version": version}
	if version == 0 {
		delete(filter, "wishlist.version")
		filter["$or"] = []bson.M{{"wishlist.version": 0}, {"wishlist.version": bson.M{"$exists": false}}}
	}
	if public {
		filter["paymentStatus"] = "paid"
		filter["isDemo"] = bson.M{"$ne": true}
		filter["publishedAt"] = bson.M{"$ne": nil}
		filter["wishlist.enabled"] = true
	}
	result, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"wishlist": next, "updatedAt": time.Now().UTC()}})
	return err == nil && result.MatchedCount == 1, err
}
