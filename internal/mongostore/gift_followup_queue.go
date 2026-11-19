package mongostore

import (
	"context"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// A list can become available after its first RSVP. Queue missing confirmations
// without resetting a send claim or a known/unknown provider outcome.
func (s Responses) QueueMissingGiftFollowups(ctx context.Context, eventID string, now time.Time) error {
	e, err := s.Snapshot(ctx, eventID)
	if err != nil {
		return err
	}
	if core.EventClosed(e, now) || e.IsDemo || !core.GiftListAvailable(e) {
		return nil
	}
	for id, response := range e.Responses {
		if response.Choice != "going" || e.GiftFollowups[id] != "" {
			continue
		}
		filter := bson.M{"_id": eventID, "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}, "deletionRequested": bson.M{"$ne": true}, "celebration.giftsEnabled": bson.M{"$ne": false}, "wishlist.items.0": bson.M{"$exists": true}, "responses." + id + ".choice": "going", "giftFollowups." + id: bson.M{"$in": bson.A{nil, ""}}}
		if _, err = s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"giftFollowups." + id: "pending_link"}, "$inc": bson.M{"responseVersion": 1}}); err != nil {
			return err
		}
	}
	return nil
}
