package mongostore

import (
	"context"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type EmailCandidate struct {
	ID             string                        `bson:"_id"`
	Name           string                        `bson:"name"`
	Email          string                        `bson:"email"`
	Title          string                        `bson:"title"`
	RequesterName  string                        `bson:"requesterName"`
	RequesterEmail string                        `bson:"requesterEmail"`
	Mail           map[string]core.EmailDelivery `bson:"mail"`
}
type Emails struct{ DB *mongo.Database }

func (s Emails) Claim(ctx context.Context, collection, kind, lease string, now time.Time) (EmailCandidate, error) {
	path := "mail." + kind
	var candidate EmailCandidate
	_, err := s.DB.Collection(collection).UpdateMany(ctx, bson.M{path + ".state": bson.M{"$in": bson.A{"pending", "sending"}}, path + ".deadline": bson.M{"$lte": now}}, bson.M{"$set": bson.M{path + ".state": "expired"}, "$unset": bson.M{path + ".payload": 1}})
	if err != nil {
		return candidate, err
	}
	filter := bson.M{path + ".deadline": bson.M{"$gt": now}, "$or": bson.A{bson.M{path + ".state": "pending", path + ".nextAt": bson.M{"$lte": now}}, bson.M{path + ".state": "sending", path + ".leaseUntil": bson.M{"$lte": now}}}}
	update := bson.M{"$set": bson.M{path + ".state": "sending", path + ".lease": lease, path + ".leaseUntil": now.Add(time.Minute)}, "$inc": bson.M{path + ".attempts": 1}}
	err = s.DB.Collection(collection).FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After).SetSort(bson.D{{Key: path + ".nextAt", Value: 1}}).SetProjection(bson.M{"_id": 1, "name": 1, "email": 1, "title": 1, "requesterName": 1, "requesterEmail": 1, path: 1})).Decode(&candidate)
	return candidate, err
}

func EnsureEmailIndexes(ctx context.Context, db *mongo.Database) error {
	for _, job := range [][2]string{{"users", "welcome"}, {"supportTickets", "ticket_received"}, {"supportTickets", "ticket_resolved"}} {
		path := "mail." + job[1]
		_, err := db.Collection(job[0]).Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: path + ".state", Value: 1}, {Key: path + ".nextAt", Value: 1}, {Key: path + ".leaseUntil", Value: 1}}})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s Emails) Prepare(ctx context.Context, collection, kind, id, lease string, payload core.EmailPayload) error {
	path := "mail." + kind
	result, err := s.DB.Collection(collection).UpdateOne(ctx, bson.M{"_id": id, path + ".lease": lease, path + ".payload": nil}, bson.M{"$set": bson.M{path + ".payload": payload}})
	if err == nil && result.MatchedCount != 1 {
		return mongo.ErrNoDocuments
	}
	return err
}

func (s Emails) Finish(ctx context.Context, collection, kind, id, lease, state string, next time.Time) error {
	path := "mail." + kind
	unset := bson.M{path + ".lease": 1, path + ".leaseUntil": 1}
	if state == "sent" {
		unset[path+".payload"] = 1
	}
	_, err := s.DB.Collection(collection).UpdateOne(ctx, bson.M{"_id": id, path + ".lease": lease}, bson.M{"$set": bson.M{path + ".state": state, path + ".nextAt": next}, "$unset": unset})
	return err
}
