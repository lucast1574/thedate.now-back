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

type AccountDeletion struct{ DB *mongo.Database }

// Persist intent before effects: a restarted worker resumes the same deletion.
func (s AccountDeletion) Request(ctx context.Context, id string) error {
	_, err := s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"deletionRequested": true}, "$unset": bson.M{"mail": ""}})
	return err
}

func (s AccountDeletion) Claim(ctx context.Context, id string) (core.User, string, error) {
	claim := uuid.NewString()
	filter := bson.M{"deletionRequested": true, "$or": bson.A{bson.M{"deletionLeaseUntil": nil}, bson.M{"deletionLeaseUntil": bson.M{"$lte": time.Now().UTC()}}}}
	if id != "" {
		filter["_id"] = id
	}
	filter["$and"] = bson.A{bson.M{"$or": bson.A{bson.M{"deletionNextAttempt": nil}, bson.M{"deletionNextAttempt": bson.M{"$lte": time.Now().UTC()}}}}}
	var u core.User
	err := s.DB.Collection("users").FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"deletionClaim": claim, "deletionLeaseUntil": time.Now().UTC().Add(3 * time.Minute)}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&u)
	return u, claim, err
}

func (s AccountDeletion) Release(ctx context.Context, id, claim string) {
	_, _ = s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": id, "deletionClaim": claim}, bson.M{"$unset": bson.M{"deletionClaim": "", "deletionLeaseUntil": ""}, "$set": bson.M{"deletionNextAttempt": time.Now().UTC().Add(30 * time.Second)}})
}

func (s AccountDeletion) Events(ctx context.Context, id string) ([]core.Event, error) {
	filter := bson.M{"ownerId": id}
	if _, err := s.DB.Collection("events").UpdateMany(ctx, filter, bson.M{"$set": bson.M{"deletionRequested": true, "invitationsEnabled": false, "invitationWebReady": false}}); err != nil {
		return nil, err
	}
	cur, err := s.DB.Collection("events").Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var events []core.Event
	err = cur.All(ctx, &events)
	return events, err
}

func (s AccountDeletion) RemoveEvent(ctx context.Context, e core.Event) error {
	for _, name := range []string{"guests", "coupleInvites", "deletedGuestBackups", "deletedEventBackups", "responseNotices", "mobileDeliveries"} {
		if _, err := s.DB.Collection(name).DeleteMany(ctx, bson.M{"eventId": e.ID}); err != nil {
			return err
		}
	}
	if _, err := s.DB.Collection("deletedEventBackups").DeleteOne(ctx, bson.M{"_id": e.ID}); err != nil {
		return err
	}
	if _, err := s.DB.Collection("deployments").DeleteOne(ctx, bson.M{"_id": e.ID}); err != nil {
		return err
	}
	_, err := s.DB.Collection("events").DeleteOne(ctx, bson.M{"_id": e.ID, "ownerId": e.OwnerID, "deletionRequested": true})
	return err
}

func (s AccountDeletion) Finish(ctx context.Context, u core.User, claim string) error {
	// Remove memberships and pending email invitations, preserving other owners' events.
	pipeline := mongo.Pipeline{bson.D{{Key: "$set", Value: bson.M{
		"coupleUserIds": bson.M{"$filter": bson.M{"input": bson.M{"$ifNull": bson.A{"$coupleUserIds", bson.A{}}}, "as": "id", "cond": bson.M{"$ne": bson.A{"$$id", u.ID}}}},
		"coupleInvites": bson.M{"$arrayToObject": bson.M{"$filter": bson.M{"input": bson.M{"$objectToArray": bson.M{"$ifNull": bson.A{"$coupleInvites", bson.M{}}}}, "as": "invite", "cond": bson.M{"$ne": bson.A{"$$invite.v.email", bson.M{"$literal": u.Email}}}}}},
		"coupleVersion": bson.M{"$add": bson.A{bson.M{"$ifNull": bson.A{"$coupleVersion", 0}}, 1}},
	}}}}
	invitedEmails := bson.M{"$map": bson.M{"input": bson.M{"$objectToArray": bson.M{"$ifNull": bson.A{"$coupleInvites", bson.M{}}}}, "as": "invite", "in": "$$invite.v.email"}}
	filter := bson.M{"$or": bson.A{bson.M{"coupleUserIds": u.ID}, bson.M{"$expr": bson.M{"$in": bson.A{bson.M{"$literal": u.Email}, invitedEmails}}}}}
	if _, err := s.DB.Collection("events").UpdateMany(ctx, filter, pipeline); err != nil {
		return err
	}
	devices := []string{}
	err := s.DB.Collection("mobileDevices").Distinct(ctx, "_id", bson.M{"userId": u.ID}).Decode(&devices)
	if err != nil {
		return err
	}
	deletes := []struct {
		name   string
		filter bson.M
	}{
		{"mobileDeliveries", bson.M{"deviceId": bson.M{"$in": devices}}},
		{"mobileDevices", bson.M{"userId": u.ID}}, {"profilePhotos", bson.M{"_id": u.ID}},
		{"supportTickets", bson.M{"requesterId": u.ID}}, {"coupleInvites", bson.M{"email": u.Email}},
		{"adminAudit", bson.M{"$or": bson.A{bson.M{"actorId": u.ID}, bson.M{"targetId": u.ID}}}},
	}
	for _, d := range deletes {
		if _, err = s.DB.Collection(d.name).DeleteMany(ctx, d.filter); err != nil {
			return err
		}
	}
	if _, err = s.DB.Collection("users").UpdateMany(ctx, bson.M{"referredBy": u.ID}, bson.M{"$unset": bson.M{"referredBy": ""}}); err != nil {
		return err
	}
	result, err := s.DB.Collection("users").DeleteOne(ctx, bson.M{"_id": u.ID, "deletionClaim": claim, "deletionRequested": true})
	if err == nil && result.DeletedCount != 1 {
		return mongo.ErrNoDocuments
	}
	return err
}
