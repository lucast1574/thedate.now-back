package mongostore

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s Deployments) RendererImage(ctx context.Context, fallback string) string {
	var release struct {
		Image string `bson:"image"`
	}
	if s.DB.Collection("runtimeConfig").FindOne(ctx, bson.M{"_id": "renderer"}).Decode(&release) == nil && release.Image != "" {
		return release.Image
	}
	return fallback
}
func (s Deployments) RollRenderer(ctx context.Context, image string) error {
	_, err := s.DB.Collection("runtimeConfig").UpdateOne(ctx, bson.M{"_id": "renderer"}, bson.M{"$set": bson.M{"image": image}}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return err
	}
	// Reconcile unclaimed jobs only; preserve resource IDs, tokens, publication, RSVP and seating.
	_, err = s.DB.Collection("deployments").UpdateMany(ctx, bson.M{"phase": bson.M{"$in": bson.A{"ready", "pending", "configured", "deploying"}}, "image": bson.M{"$ne": image}, "claim": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"image": image, "phase": "pending", "finalized": false}, "$unset": bson.M{"error": "", "nextAttemptAt": ""}})
	return err
}
