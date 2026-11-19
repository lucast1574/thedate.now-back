package mongostore

import (
	"context"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func queueConfirmationFollowups(set bson.M, e core.Event, id string, r core.Response) {
	if core.GiftsEnabled(e) && e.GiftFollowups[id] == "" {
		set["giftFollowups."+id] = "pending_link"
	}
	if r.Seats > 1 && !r.PartyRegistered && e.ConfirmationFollowups["companions"][id] == "" {
		set["confirmationFollowups.companions."+id] = "pending"
	}
	payment := e.ConfirmationFollowups["payment"][id]
	if e.Kind == "general" && e.Admission != nil && e.Admission.Enabled && (payment == "" || payment == "skipped") {
		set["confirmationFollowups.payment."+id] = "pending"
	}
}

func (s Responses) ClaimConfirmationFollowup(ctx context.Context, e core.Event, g core.Guest, kind string) (bool, error) {
	field := "confirmationFollowups." + kind + "." + g.ID
	filter := bson.M{
		"_id": e.ID, "responseVersion": e.ResponseVersion, field: "pending",
		"responses." + g.ID + ".choice": "going", "revokedGuestTokens." + g.InviteToken: bson.M{"$ne": true},
		"deletionRequested": bson.M{"$ne": true}, "publishedAt": bson.M{"$ne": nil}, "paymentStatus": "paid",
	}
	result, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{field: "sending"}, "$inc": bson.M{"responseVersion": 1}})
	return err == nil && result.MatchedCount == 1, err
}

func (s Responses) FinishConfirmationFollowup(ctx context.Context, event, guest, kind, state string) error {
	field := "confirmationFollowups." + kind + "." + guest
	_, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": event, field: "sending"}, bson.M{"$set": bson.M{field: state}})
	return err
}
