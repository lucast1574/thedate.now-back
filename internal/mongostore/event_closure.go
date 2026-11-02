package mongostore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s Deployments) ClaimClosure(ctx context.Context, id string, now time.Time) (core.Deployment, string, error) {
	var d core.Deployment
	claim := uuid.NewString()
	filter := bson.M{"_id": id, "phase": bson.M{"$ne": "stopped"}, "$or": bson.A{bson.M{"leaseUntil": nil}, bson.M{"leaseUntil": bson.M{"$lte": now}}}}
	err := s.DB.Collection("deployments").FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"claim": claim, "leaseUntil": now.Add(3 * time.Minute)}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&d)
	return d, claim, err
}
