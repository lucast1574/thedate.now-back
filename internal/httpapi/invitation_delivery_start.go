package httpapi

import (
	"net/http"
	"os"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/dokploy"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *server) startDelivery(w http.ResponseWriter, r *http.Request) {
	s.beginDelivery(w, r, false)
}

func (s *server) beginDelivery(w http.ResponseWriter, r *http.Request, retry bool) {
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 403, "Activa tu evento antes de enviar invitaciones.")
		return
	}
	if !e.StartAt.After(time.Now().UTC()) {
		bad(w, 409, "La fecha del evento ya pasó. Puedes seguir consultando las respuestas.")
		return
	}
	if !wazendSessionReady(r.Context(), wazendFor(e.Kind)) {
		bad(w, 503, "El envío por WhatsApp no está disponible en este momento. Tu lista está guardada.")
		return
	}
	count, err := s.db.Collection("guests").CountDocuments(r.Context(), bson.M{"eventId": e.ID})
	if err != nil {
		bad(w, 500, "No pudimos consultar tu lista. Vuelve a intentarlo.")
		return
	}
	if count == 0 {
		bad(w, 409, "Añade al menos un invitado antes de enviar.")
		return
	}
	ready := s.invitationPageReady(r.Context(), e)
	if !ready {
		image := (mongostore.Deployments{DB: s.db}).RendererImage(r.Context(), os.Getenv("DOKPLOY_INVITATION_IMAGE"))
		if _, err = deploymentClientFor(s.product); err != nil || !dokploy.ValidImage(image) {
			bad(w, 503, "No podemos preparar la web en este momento. Tu lista está guardada.")
			return
		}
		if _, err = (mongostore.Deployments{DB: s.db}).Queue(r.Context(), e, image); err != nil {
			bad(w, 500, "No pudimos preparar tu invitación. Vuelve a intentarlo.")
			return
		}
	}
	if err = (mongostore.InvitationQueue{DB: s.db}).PreserveSentOffers(r.Context(), e.ID); err != nil {
		bad(w, 500, "No pudimos preparar el orden de envío. Tu lista está guardada.")
		return
	}
	if retry {
		if err = s.retryRejectedInvitations(r.Context(), e.ID); err != nil {
			bad(w, 500, "No pudimos preparar los reintentos. Tu lista est\u00e1 guardada.")
			return
		}
	}
	_, err = s.db.Collection("events").UpdateOne(r.Context(), bson.M{"_id": e.ID, "paymentStatus": "paid"}, bson.M{"$set": bson.M{"invitationsEnabled": true, "invitationWebReady": ready}})
	if err != nil {
		bad(w, 500, "No pudimos iniciar el envío. Vuelve a intentarlo.")
		return
	}
	e.InvitationsEnabled = true
	e.InvitationWebReady = ready
	reply(w, http.StatusAccepted, s.deliveryView(r.Context(), e))
}
