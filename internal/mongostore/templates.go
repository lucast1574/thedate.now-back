package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func EnsureTemplates(ctx context.Context, db *mongo.Database) error {
	for _, t := range core.TemplateCatalog() {
		if _, err := db.Collection("invitationTemplates").ReplaceOne(ctx, bson.M{"_id": t.ID}, t, options.Replace().SetUpsert(true)); err != nil {
			return err
		}
	}
	// Backfill only legacy records without a selected template; never replace a saved choice.
	for _, kind := range []string{"wedding", "general"} {
		for _, style := range []string{"classic", "modern"} {
			for _, mode := range []string{"sections", "flyer"} {
				filter := bson.M{"kind": kind, "template": style, "templateId": bson.M{"$in": bson.A{nil, ""}}}
				if mode == "sections" {
					filter["designMode"] = bson.M{"$in": bson.A{nil, "", "sections"}}
				} else {
					filter["designMode"] = mode
				}
				if _, err := db.Collection("events").UpdateMany(ctx, filter, bson.M{"$set": bson.M{"templateId": core.TemplateID(kind, style, mode, "")}}); err != nil {
					return err
				}
			}
		}
	}
	_, err := db.Collection("invitationTemplates").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "mode", Value: 1}}})
	return err
}
