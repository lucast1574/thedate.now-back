package mongostore

import (
	"context"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"strings"
)

func AffiliateCode() string { return strings.ReplaceAll(uuid.NewString(), "-", "")[:24] }

// EnableDefaults preserves existing links, balances and payment/withdrawal markers.
// Conditional writes keep concurrent sign-ins from replacing someone's link.
func (s Affiliates) EnableDefaults(ctx context.Context, id string) (Wallet, error) {
	_, err := s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": id, "role": bson.M{"$ne": "admin"}, "affiliateCode": bson.M{"$in": bson.A{nil, ""}}}, bson.M{"$set": bson.M{"affiliateCode": AffiliateCode(), "affiliateEnabled": true}})
	if err != nil {
		return Wallet{}, err
	}
	_, err = s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": id, "role": bson.M{"$ne": "admin"}, "affiliateEnabled": bson.M{"$ne": true}}, bson.M{"$set": bson.M{"affiliateEnabled": true}})
	if err != nil {
		return Wallet{}, err
	}
	return s.Wallet(ctx, id)
}

func EnsureAffiliateDefaults(ctx context.Context, db *mongo.Database) error {
	cursor, err := db.Collection("users").Find(ctx, bson.M{"role": bson.M{"$ne": "admin"}, "$or": bson.A{bson.M{"affiliateEnabled": bson.M{"$ne": true}}, bson.M{"affiliateCode": bson.M{"$in": bson.A{nil, ""}}}}})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var account struct {
			ID string `bson:"_id"`
		}
		if err := cursor.Decode(&account); err != nil {
			return err
		}
		if _, err := (Affiliates{DB: db}).EnableDefaults(ctx, account.ID); err != nil {
			return err
		}
	}
	return cursor.Err()
}
