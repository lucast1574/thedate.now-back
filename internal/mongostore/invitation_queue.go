package mongostore

import (
	"context"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type InvitationQueue struct{ DB *mongo.Database }

// Existing sent invitations must keep their seats when a legacy event adopts the queue.
func (s InvitationQueue) PreserveSentOffers(ctx context.Context, eventID string) error {
	for attempt := 0; attempt < 32; attempt++ {
		e, err := (Responses{DB: s.DB}).Snapshot(ctx, eventID)
		if err != nil {
			return err
		}
		cur, err := s.DB.Collection("guests").Find(ctx, bson.M{"eventId": eventID, "sentAt": bson.M{"$ne": nil}})
		if err != nil {
			return err
		}
		var guests []core.Guest
		err = cur.All(ctx, &guests)
		cur.Close(ctx)
		if err != nil {
			return err
		}
		set := bson.M{}
		for _, g := range guests {
			r := e.Responses[g.ID]
			if r.Choice == "" || r.Choice == "pending" {
				set["invitationHolds."+g.ID] = g.Seats
			}
		}
		if len(set) == 0 {
			return nil
		}
		result, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": eventID, "responseVersion": e.ResponseVersion}, bson.M{"$set": set, "$inc": bson.M{"responseVersion": 1}})
		if err != nil || result.MatchedCount == 1 {
			return err
		}
	}
	return mongo.ErrNoDocuments
}

func (s InvitationQueue) Allocate(ctx context.Context, eventID string, count int) (int64, error) {
	var counter struct {
		Next int64 `bson:"guestQueueNext"`
	}
	err := s.DB.Collection("events").FindOneAndUpdate(ctx, bson.M{"_id": eventID}, bson.M{"$inc": bson.M{"guestQueueNext": count}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&counter)
	return counter.Next - int64(count) + 1, err
}

func GuestQueueSort() bson.D {
	return bson.D{{Key: "queuePosition", Value: 1}, {Key: "createdAt", Value: 1}, {Key: "_id", Value: 1}}
}

// Reserve in the same event/version as RSVP, so replicas cannot overbook.
func (s InvitationQueue) Reserve(ctx context.Context, eventID string, g core.Guest, now time.Time) (bool, error) {
	for attempt := 0; attempt < 32; attempt++ {
		e, err := (Responses{DB: s.DB}).Snapshot(ctx, eventID)
		if err != nil || !core.CanOfferInvitation(e, g, now) {
			return false, err
		}
		if e.InvitationHolds[g.ID] > 0 {
			return true, nil
		}
		result, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": eventID, "responseVersion": e.ResponseVersion, "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}}, bson.M{"$set": bson.M{"invitationHolds." + g.ID: g.Seats}, "$inc": bson.M{"responseVersion": 1}})
		if err != nil || result.MatchedCount == 1 {
			return err == nil, err
		}
	}
	return false, nil
}

func (s InvitationQueue) Release(ctx context.Context, eventID, guestID string) error {
	_, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": eventID}, bson.M{"$unset": bson.M{"invitationHolds." + guestID: ""}, "$inc": bson.M{"responseVersion": 1}})
	return err
}
