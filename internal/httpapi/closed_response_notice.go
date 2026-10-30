package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

func (s *server) notifyClosedResponse(ctx context.Context, eventID string, g core.Guest, choice, receipt string, timestamp int64) error {
	e, err := (mongostore.Responses{DB: s.db}).Snapshot(ctx, eventID)
	if err != nil {
		return err
	}
	prior := e.Responses[g.ID]
	if e.RevokedGuestTokens[g.InviteToken] || !core.ResponseCompleted(prior) || prior.Choice == choice || (timestamp > 0 && prior.ProviderTimestamp >= timestamp) {
		return nil
	}
	// Claim before contacting WhatsApp: a repeated webhook must never repeat a notice.
	_, err = s.db.Collection("responseNotices").InsertOne(ctx, bson.M{"_id": receipt, "eventId": eventID, "guestId": g.ID, "createdAt": time.Now().UTC()})
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	if err != nil {
		return err
	}
	contact := "quienes organizan la celebración"
	if e.Kind == "wedding" {
		contact = "los novios"
	}
	requestRSVPFollowup(e, g, "Tu respuesta ya está registrada. El cambio en la encuesta no modifica tu confirmación. Para solicitar un cambio, comunícate con "+contact+". Puedes consultar tu respuesta aquí: ")
	return nil
}
