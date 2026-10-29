package mongostore

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"strings"
)

func EnsurePaidAddressIndex(ctx context.Context, db *mongo.Database) error {
	indexes := db.Collection("events").Indexes()
	_, err := indexes.CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "slug", Value: 1}}, Options: options.Index().SetName("paid_invitation_address").SetUnique(true).SetPartialFilterExpression(bson.M{"paymentStatus": "paid"})})
	if err != nil {
		return err
	}
	cursor, err := indexes.List(ctx)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var spec struct {
			Name    string `bson:"name"`
			Unique  bool   `bson:"unique"`
			Partial bson.M `bson:"partialFilterExpression"`
			Key     bson.D `bson:"key"`
		}
		if err = cursor.Decode(&spec); err != nil {
			return err
		}
		if spec.Name == "kind_1_slug_1" && spec.Unique && len(spec.Partial) == 0 && len(spec.Key) == 2 && spec.Key[0].Key == "kind" && spec.Key[1].Key == "slug" {
			err = indexes.DropOne(ctx, spec.Name)
			if err != nil {
				var command mongo.CommandError
				if !errors.As(err, &command) || (command.Code != 27 && command.Code != 26) {
					return err
				}
			}
		}
	}
	return cursor.Err()
}
func PaidAddressFallback(e core.Event) string {
	id := strings.ReplaceAll(e.ID, "-", "")
	if len(id) > 32 {
		id = id[:32]
	}
	return "event-" + id
}
