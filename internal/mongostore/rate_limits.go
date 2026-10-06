package mongostore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"strconv"
	"time"
)

func Allow(ctx context.Context, db *mongo.Database, key string, limit int, now time.Time) (bool, error) {
	hash := sha256.Sum256([]byte(key + ":" + strconv.FormatInt(now.Unix()/60, 10)))
	var bucket struct {
		Count int `bson:"count"`
	}
	err := db.Collection("rateLimits").FindOneAndUpdate(ctx, bson.M{"_id": hex.EncodeToString(hash[:])}, bson.M{"$inc": bson.M{"count": 1}, "$setOnInsert": bson.M{"expiresAt": now.Add(2 * time.Minute)}}, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&bucket)
	if err != nil {
		return false, err
	}
	return bucket.Count <= limit, nil
}
