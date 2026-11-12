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
	filter := bson.M{"_id": id, "$or": bson.A{bson.M{"leaseUntil": nil}, bson.M{"leaseUntil": bson.M{"$lte": now}}}}
	err := s.DB.Collection("deployments").FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"claim": claim, "leaseUntil": now.Add(3 * time.Minute)}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&d)
	return d, claim, err
}

// Release only after the provider route is removed; keep the original slug for records.
func (s Deployments) ReleaseAddress(ctx context.Context, e core.Event, now time.Time) error {
	if !core.EventClosed(e, now) || e.IsDemo {
		return core.ErrUnavailable
	}
	_, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": e.ID, "slug": e.Slug, "startAt": e.StartAt, "timeZone": e.TimeZone, "addressReleasedAt": nil}, bson.M{"$set": bson.M{"addressReleasedAt": now, "invitationsEnabled": false, "invitationWebReady": false}})
	return err
}
