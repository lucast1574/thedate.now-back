package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Seating struct{ DB *mongo.Database }

func (s Seating) Save(ctx context.Context, e core.Event, plan core.SeatingPlan) (bool, error) {
	filter := bson.M{"_id": e.ID, "responseVersion": e.ResponseVersion, "paymentStatus": "paid"}
	if plan.Version == 0 {
		filter["seating.version"] = bson.M{"$in": bson.A{nil, 0}}
	} else {
		filter["seating.version"] = plan.Version
	}
	plan.Version++
	result, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"seating": plan}, "$inc": bson.M{"responseVersion": 1}})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}
