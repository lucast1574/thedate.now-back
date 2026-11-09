package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"time"
)

func (s Responses) ClaimGiftFollowup(ctx context.Context, e core.Event, g core.Guest, now time.Time) (bool, error) {
	if core.EventClosed(e, now) || e.IsDemo || g.InviteToken == "" || e.RevokedGuestTokens[g.InviteToken] {
		return false, nil
	}
	filter := bson.M{"_id": e.ID, "paymentStatus": "paid", "isDemo": bson.M{"$ne": true}, "publishedAt": bson.M{"$ne": nil}, "giftFollowups." + g.ID: "pending", "responses." + g.ID + ".choice": "going", "revokedGuestTokens." + g.InviteToken: bson.M{"$ne": true}}
	result, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"giftFollowups." + g.ID: "sending"}, "$inc": bson.M{"responseVersion": 1}})
	return err == nil && result.MatchedCount == 1, err
}
func (s Responses) FinishGiftFollowup(ctx context.Context, eventID, guestID, state string) error {
	_, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": eventID, "giftFollowups." + guestID: "sending"}, bson.M{"$set": bson.M{"giftFollowups." + guestID: state}})
	return err
}
