package mongostore

import (
	"context"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"
)

type Deployments struct{ DB *mongo.Database }

func (s Deployments) Queue(ctx context.Context, e core.Event, image string) (core.Deployment, error) {
	host, err := core.InvitationHost(e.Kind, e.Slug)
	if err != nil {
		return core.Deployment{}, err
	}
	d := core.Deployment{EventID: e.ID, Host: host, Image: image, Phase: "pending"}
	_, err = s.DB.Collection("deployments").UpdateOne(ctx, bson.M{"_id": e.ID}, bson.M{"$setOnInsert": d}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return d, err
	}
	// Explicit publish retries failed jobs, preserving checkpoints/resources.
	_, err = s.DB.Collection("deployments").UpdateOne(ctx, bson.M{"_id": e.ID, "phase": "error"}, bson.M{"$set": bson.M{"phase": "pending", "error": "", "image": image}})
	if err != nil {
		return d, err
	}
	return s.Get(ctx, e.ID)
}
func (s Deployments) Get(ctx context.Context, id string) (core.Deployment, error) {
	var d core.Deployment
	err := s.DB.Collection("deployments").FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	return d, err
}
func (s Deployments) Claim(ctx context.Context) (core.Deployment, string, error) {
	now, claim := time.Now().UTC(), uuid.NewString()
	var d core.Deployment
	filter := bson.M{"phase": bson.M{"$in": bson.A{"pending", "configured", "deploying", "ready"}}, "finalized": bson.M{"$ne": true}, "$or": bson.A{bson.M{"leaseUntil": bson.M{"$lte": now}}, bson.M{"leaseUntil": nil}}, "$and": bson.A{bson.M{"$or": bson.A{bson.M{"nextAttemptAt": bson.M{"$lte": now}}, bson.M{"nextAttemptAt": nil}}}}}
	err := s.DB.Collection("deployments").FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"claim": claim, "leaseUntil": now.Add(3 * time.Minute)}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&d)
	return d, claim, err
}
func (s Deployments) Save(ctx context.Context, d core.Deployment, claim string) error {
	result, err := s.DB.Collection("deployments").UpdateOne(ctx, bson.M{"_id": d.EventID, "claim": claim, "leaseUntil": bson.M{"$gt": time.Now().UTC()}}, bson.M{"$set": bson.M{"projectId": d.ProjectID, "environmentId": d.EnvironmentID, "applicationId": d.ApplicationID, "domainId": d.DomainID, "phase": d.Phase, "error": d.Error}})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return mongo.ErrNoDocuments
	}
	return nil
}
func (s Deployments) Finish(ctx context.Context, d core.Deployment, claim string) error {
	var current core.Deployment
	if err := s.DB.Collection("deployments").FindOne(ctx, bson.M{"_id": d.EventID, "claim": claim, "phase": d.Phase, "leaseUntil": bson.M{"$gt": time.Now().UTC()}}).Decode(&current); err != nil {
		return err
	}
	if d.Phase == "ready" {
		now := time.Now().UTC()
		result, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": d.EventID, "paymentStatus": "paid"}, mongo.Pipeline{bson.D{{Key: "$set", Value: bson.M{"publishedAt": bson.M{"$ifNull": bson.A{"$publishedAt", now}}, "updatedAt": now}}}})
		if err != nil {
			return err
		}
		if result.MatchedCount != 1 {
			return mongo.ErrNoDocuments
		}
	}
	update := bson.M{"$unset": bson.M{"claim": "", "leaseUntil": ""}, "$set": bson.M{"nextAttemptAt": time.Now().UTC().Add(10 * time.Second), "finalized": d.Phase == "ready"}}
	result, err := s.DB.Collection("deployments").UpdateOne(ctx, bson.M{"_id": d.EventID, "claim": claim}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return mongo.ErrNoDocuments
	}
	return nil
}
