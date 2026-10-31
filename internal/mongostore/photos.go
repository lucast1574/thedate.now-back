package mongostore

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

func AttachPhoto(ctx context.Context, db *mongo.Database, id, key string, now time.Time) (bool, error) {
	result, err := db.Collection("events").UpdateOne(ctx, bson.M{"_id": id, "photoKeys.39": bson.M{"$exists": false}}, mongo.Pipeline{
		preservePublishedStage(),
		bson.D{{Key: "$set", Value: bson.M{"photoKeys": bson.M{"$concatArrays": bson.A{bson.M{"$ifNull": bson.A{"$photoKeys", bson.A{}}}, bson.A{key}}}, "updatedAt": now, "designVersion": bson.M{"$add": bson.A{bson.M{"$ifNull": bson.A{"$designVersion", 0}}, 1}}}}},
	})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}
