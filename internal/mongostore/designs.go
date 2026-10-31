package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

type Designs struct{ DB *mongo.Database }

func designVersionFilter(e core.Event) bson.M {
	filter := bson.M{"_id": e.ID, "designVersion": e.DesignVersion, "publishedAt": e.PublishedAt}
	if e.DesignVersion == 0 {
		filter["designVersion"] = bson.M{"$in": bson.A{nil, 0}}
	}
	return filter
}

// Called in the same conditional write as any change to editable design fields.
func preservePublishedDesign(e core.Event, set bson.M) {
	if e.PublishedAt != nil && e.PublishedDesign == nil {
		set["publishedDesign"] = core.SnapshotDesign(e)
		set["publishedDesignVersion"] = e.DesignVersion
	}
}

func (s Designs) Save(ctx context.Context, e, next core.Event, now time.Time) (bool, error) {
	raw, err := bson.Marshal(core.SnapshotDesign(next).DesignInput)
	if err != nil {
		return false, err
	}
	var set bson.M
	if err = bson.Unmarshal(raw, &set); err != nil {
		return false, err
	}
	set["updatedAt"] = now
	preservePublishedDesign(e, set)
	result, err := s.DB.Collection("events").UpdateOne(ctx, designVersionFilter(e), bson.M{"$set": set, "$inc": bson.M{"designVersion": 1}})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}

func (s Designs) Publish(ctx context.Context, e core.Event, now time.Time) (bool, error) {
	filter := designVersionFilter(e)
	filter["paymentStatus"], filter["isDemo"] = "paid", bson.M{"$ne": true}
	result, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"publishedDesign": core.SnapshotDesign(e), "publishedDesignVersion": e.DesignVersion, "updatedAt": now}})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}
