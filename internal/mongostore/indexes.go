package mongostore

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		return err
	}
	_, err = db.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "googleSub", Value: 1}}, Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"googleSub": bson.M{"$exists": true}})})
	if err != nil {
		return err
	}
	_, err = db.Collection("events").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		return err
	}
	_, err = db.Collection("guests").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "inviteToken", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		return err
	}
	for collection, indexes := range map[string][]mongo.IndexModel{
		"events":        {{Keys: bson.D{{Key: "ownerId", Value: 1}}}, {Keys: bson.D{{Key: "coupleUserIds", Value: 1}}}},
		"guests":        {{Keys: bson.D{{Key: "eventId", Value: 1}, {Key: "sentAt", Value: 1}}}, {Keys: bson.D{{Key: "invitationMessageId", Value: 1}}}},
		"coupleInvites": {{Keys: bson.D{{Key: "tokenHash", Value: 1}}, Options: options.Index().SetUnique(true)}},
		"deployments":   {{Keys: bson.D{{Key: "phase", Value: 1}, {Key: "finalized", Value: 1}, {Key: "nextAttemptAt", Value: 1}, {Key: "leaseUntil", Value: 1}}}},
		"rateLimits":    {{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)}},
	} {
		if _, err = db.Collection(collection).Indexes().CreateMany(ctx, indexes); err != nil {
			return err
		}
	}
	return nil
}
