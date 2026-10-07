package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ChangeUserRole compares both role and session version, including legacy users
// whose zero session version has not yet been persisted.
func ChangeUserRole(ctx context.Context, db *mongo.Database, target core.User, role string) (bool, error) {
	filter := bson.M{"_id": target.ID, "role": target.Role, "tokenVersion": target.TokenVersion}
	if target.TokenVersion == 0 {
		filter["$or"] = bson.A{bson.M{"tokenVersion": 0}, bson.M{"tokenVersion": bson.M{"$exists": false}}}
		delete(filter, "tokenVersion")
	}
	result, err := db.Collection("users").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"role": role}, "$inc": bson.M{"tokenVersion": 1}})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}
