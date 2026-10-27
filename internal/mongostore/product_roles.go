package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Normalize retired roles without touching administrators or event memberships.
func EnsureProductRoles(ctx context.Context, db *mongo.Database, kind string) error {
	allowed := []string{"admin", "user", "planner", "organizer"}
	if kind != "" {
		p, err := core.ProductFor(kind)
		if err != nil {
			return err
		}
		allowed = []string{"admin", "user", p.Role}
	}
	_, err := db.Collection("users").UpdateMany(ctx, bson.M{"role": bson.M{"$nin": allowed}}, bson.M{"$set": bson.M{"role": "user"}, "$inc": bson.M{"tokenVersion": 1}})
	return err
}
