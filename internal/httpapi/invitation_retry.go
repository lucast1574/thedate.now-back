package httpapi

import (
	"context"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *server) retryDelivery(w http.ResponseWriter, r *http.Request) {
	s.beginDelivery(w, r, true)
}

func (s *server) retryRejectedInvitations(ctx context.Context, eventID string) error {
	twoMinutesAgo := time.Now().UTC().Add(-2 * time.Minute)
	_, err := s.db.Collection("guests").UpdateMany(ctx,
		bson.M{
			"eventId": eventID,
			"sentAt":  nil,
			"$or": []bson.M{
				{"sendState": "failed"},
				{
					"sendState":               bson.M{"$in": bson.A{"unknown", "sending"}},
					"invitationTextMessageId": bson.M{"$in": bson.A{"", nil}},
					"invitationMessageId":     bson.M{"$in": bson.A{"", nil}},
					"sendStartedAt":           bson.M{"$lt": twoMinutesAgo},
				},
			},
		},
		bson.M{"$unset": bson.M{"sendClaim": "", "sendStartedAt": "", "sendState": ""}},
	)
	return err
}
