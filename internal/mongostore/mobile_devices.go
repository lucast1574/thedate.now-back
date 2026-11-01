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

type MobileDevices struct{ DB *mongo.Database }

func (s MobileDevices) Register(ctx context.Context, d core.MobileDevice, now time.Time) error {
	// Pipeline update binds installation changes atomically, including account switches.
	same := bson.M{"$and": bson.A{bson.M{"$eq": bson.A{"$userId", d.UserID}}, bson.M{"$eq": bson.A{"$fid", d.FID}}, bson.M{"$eq": bson.A{"$tokenVersion", d.TokenVersion}}}}
	literal := func(v any) bson.M { return bson.M{"$literal": v} }
	set := bson.M{"userId": literal(d.UserID), "fid": literal(d.FID), "platform": literal(d.Platform), "tokenVersion": d.TokenVersion, "seenAt": now,
		"registeredAt": bson.M{"$cond": bson.A{same, "$registeredAt", now}}, "binding": bson.M{"$cond": bson.A{same, "$binding", uuid.NewString()}}}
	_, err := s.DB.Collection("mobileDevices").UpdateOne(ctx, bson.M{"_id": d.ID}, mongo.Pipeline{bson.D{{Key: "$set", Value: set}}}, options.UpdateOne().SetUpsert(true))
	return err
}
func (s MobileDevices) Remove(ctx context.Context, id, user string) error {
	_, err := s.DB.Collection("mobileDevices").DeleteOne(ctx, bson.M{"_id": id, "userId": user})
	return err
}
func (s MobileDevices) Page(ctx context.Context, after string, now time.Time) ([]core.MobileDevice, error) {
	cur, err := s.DB.Collection("mobileDevices").Find(ctx, bson.M{"_id": bson.M{"$gt": after}, "seenAt": bson.M{"$gt": now.AddDate(0, 0, -30)}}, options.Find().SetLimit(50).SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	devices := []core.MobileDevice{}
	err = cur.All(ctx, &devices)
	return devices, err
}
func (s MobileDevices) Claim(ctx context.Context, d core.MobileDevice, change core.MobileChange, now time.Time) (string, error) {
	// Unique _id is the cross-process claim. Ambiguous sends are never replayed.
	id := d.ID + ":" + d.Binding + ":" + change.ID
	_, err := s.DB.Collection("mobileDeliveries").InsertOne(ctx, bson.M{"_id": id, "deviceId": d.ID, "binding": d.Binding, "eventId": change.EventID, "state": "sending", "createdAt": now, "expiresAt": now.AddDate(0, 0, 35)})
	return id, err
}
func (s MobileDevices) Finish(ctx context.Context, id, state string) error {
	_, err := s.DB.Collection("mobileDeliveries").UpdateOne(ctx, bson.M{"_id": id, "state": "sending"}, bson.M{"$set": bson.M{"state": state}})
	return err
}
func (s MobileDevices) Current(ctx context.Context, d core.MobileDevice, eventID string) (bool, error) {
	var current core.MobileDevice
	err := s.DB.Collection("mobileDevices").FindOne(ctx, bson.M{"_id": d.ID, "binding": d.Binding}).Decode(&current)
	if err == mongo.ErrNoDocuments {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var u core.User
	var e core.Event
	if err = s.DB.Collection("users").FindOne(ctx, bson.M{"_id": d.UserID}).Decode(&u); err != nil {
		return false, err
	}
	if err = s.DB.Collection("events").FindOne(ctx, bson.M{"_id": eventID}).Decode(&e); err != nil {
		return false, err
	}
	return core.CanReceiveMobilePush(u, current, e), nil
}
func EnsureMobileIndexes(ctx context.Context, db *mongo.Database) error {
	for collection, indexes := range map[string][]mongo.IndexModel{
		"mobileDevices":    {{Keys: bson.D{{Key: "userId", Value: 1}}}, {Keys: bson.D{{Key: "seenAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(2592000)}},
		"mobileDeliveries": {{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)}},
	} {
		if _, err := db.Collection(collection).Indexes().CreateMany(ctx, indexes); err != nil {
			return err
		}
	}
	return nil
}

func (s MobileDevices) RemoveBinding(ctx context.Context, d core.MobileDevice) error {
	_, err := s.DB.Collection("mobileDevices").DeleteOne(ctx, bson.M{"_id": d.ID, "binding": d.Binding})
	return err
}
