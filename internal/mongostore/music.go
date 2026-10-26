package mongostore

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

func AttachAudio(ctx context.Context, db *mongo.Database, eventID, key string, now time.Time) (bool, error) {
	result, err := db.Collection("events").UpdateOne(ctx, bson.M{"_id": eventID, "audioKeys.19": bson.M{"$exists": false}}, bson.M{"$addToSet": bson.M{"audioKeys": key}, "$set": bson.M{"updatedAt": now}})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}
