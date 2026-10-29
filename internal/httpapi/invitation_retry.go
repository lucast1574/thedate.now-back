package httpapi

import (
	"context"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *server) retryDelivery(w http.ResponseWriter, r *http.Request) {
	s.beginDelivery(w, r, true)
}

func (s *server) retryRejectedInvitations(ctx context.Context, eventID string) error {
	// Only explicit provider rejections are safe to retry. Sent messages and unknown
	// outcomes retain their claims so a retry can never duplicate them.
	_, err := s.db.Collection("guests").UpdateMany(ctx,
		bson.M{"eventId": eventID, "sentAt": nil, "sendState": "failed"},
		bson.M{"$unset": bson.M{"sendClaim": "", "sendStartedAt": "", "sendState": ""}},
	)
	return err
}
