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
	state := e.GiftFollowups[g.ID]
	filter := bson.M{"_id": e.ID, "paymentStatus": "paid", "isDemo": bson.M{"$ne": true}, "deletionRequested": bson.M{"$ne": true}, "publishedAt": bson.M{"$ne": nil}, "giftFollowups." + g.ID: state, "responses." + g.ID + ".choice": "going", "revokedGuestTokens." + g.InviteToken: bson.M{"$ne": true}}
	giftEligible := bson.M{"celebration.giftsEnabled": bson.M{"$ne": false}}
	if state != "pending_empty" && state != "pending_link" {
		giftEligible["wishlist.items.0"] = bson.M{"$exists": true}
	}
	eligible := bson.A{giftEligible}
	sending := "sending"
	switch state {
	case "sent_empty":
		sending = "ready_sending"
	case "pending", "pending_empty", "pending_link":
		if state != "pending_link" && g.Seats > 1 {
			eligible = append(eligible, bson.M{"responses." + g.ID + ".partyRegistered": bson.M{"$ne": true}})
		}
	default:
		return false, nil
	}
	filter["$or"] = eligible
	result, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"giftFollowups." + g.ID: sending}, "$inc": bson.M{"responseVersion": 1}})
	return err == nil && result.MatchedCount == 1, err
}
func (s Responses) FinishGiftFollowup(ctx context.Context, eventID, guestID, state string) error {
	return s.finishGiftFollowup(ctx, eventID, guestID, "sending", state)
}

func (s Responses) FinishGiftReadyNotice(ctx context.Context, eventID, guestID, state string) error {
	return s.finishGiftFollowup(ctx, eventID, guestID, "ready_sending", state)
}

func (s Responses) finishGiftFollowup(ctx context.Context, eventID, guestID, expected, state string) error {
	_, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": eventID, "giftFollowups." + guestID: expected}, bson.M{"$set": bson.M{"giftFollowups." + guestID: state}})
	return err
}
