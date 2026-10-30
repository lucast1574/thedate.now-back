package mongostore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"
)

// Claim before contacting the provider, across replicas. Unknown outcomes never retry automatically.
func ClaimGiftAccess(ctx context.Context, db *mongo.Database, key string, now time.Time) (bool, error) {
	hash := sha256.Sum256([]byte("gift-access:" + key))
	filter := bson.M{"_id": "gift-access:" + hex.EncodeToString(hash[:]), "$or": []bson.M{{"nextSendAt": bson.M{"$lte": now}}, {"nextSendAt": bson.M{"$exists": false}}}}
	res, err := db.Collection("rateLimits").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"nextSendAt": now.Add(5 * time.Minute), "expiresAt": now.Add(6 * time.Minute)}}, options.UpdateOne().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil && (res.MatchedCount > 0 || res.UpsertedCount > 0), err
}
