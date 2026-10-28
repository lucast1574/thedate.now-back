package httpapi

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/dokploy"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *server) startDelivery(w http.ResponseWriter, r *http.Request) {
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
	if !wazendFor(e.Kind).ready() {
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
	_, err = s.db.Collection("events").UpdateOne(r.Context(), bson.M{"_id": e.ID, "paymentStatus": "paid"}, bson.M{"$set": bson.M{"invitationsEnabled": true, "invitationWebReady": ready}})
	if err != nil {
		bad(w, 500, "No pudimos iniciar el envío. Vuelve a intentarlo.")
		return
	}
	e.InvitationsEnabled = true
	e.InvitationWebReady = ready
	reply(w, http.StatusAccepted, s.deliveryView(r.Context(), e))
}

func (s *server) deliveryStatus(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	reply(w, 200, s.deliveryView(r.Context(), e))
}

type deliveryView struct {
	Phase     string `json:"phase"`
	Total     int    `json:"total"`
	Sent      int    `json:"sent"`
	Waiting   int    `json:"waiting"`
	Failed    int    `json:"failed"`
	Uncertain int    `json:"uncertain"`
}

func (s *server) deliveryView(ctx context.Context, e core.Event) deliveryView {
	v := deliveryView{Phase: "draft"}
	inFlight := 0
	cur, err := s.db.Collection("guests").Find(ctx, bson.M{"eventId": e.ID})
	if err != nil {
		v.Phase = "error"
		return v
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var g core.Guest
		if cur.Decode(&g) != nil {
			v.Phase = "error"
			return v
		}
		v.Total++
		switch {
		case g.SentAt != nil:
			v.Sent++
		case g.SendState == "failed":
			v.Failed++
		case core.InvitationSendState(g, time.Now().UTC()) == "unknown":
			v.Uncertain++
		case g.SendState == "sending":
			inFlight++
			v.Waiting++
		default:
			v.Waiting++
		}
	}
	if cur.Err() != nil {
		v.Phase = "error"
		return v
	}
	if !e.InvitationsEnabled {
		return v
	}
	v.Phase = "preparing"
	d, err := (mongostore.Deployments{DB: s.db}).Get(ctx, e.ID)
	if err == nil && d.Phase == "error" {
		v.Phase = "error"
		return v
	}
	if e.PublishedAt == nil || err != nil || d.Phase != "ready" || !e.InvitationWebReady {
		return v
	}
	v.Phase = "sending"
	if v.Waiting == 0 {
		v.Phase = "complete"
	}
	if v.Failed+v.Uncertain > 0 {
		v.Phase = "attention"
	}
	if v.Waiting > 0 {
		latest, err := (mongostore.Responses{DB: s.db}).Snapshot(ctx, e.ID)
		var next core.Guest
		if err == nil && s.db.Collection("guests").FindOne(ctx, bson.M{"eventId": e.ID, "sentAt": nil, "sendClaim": bson.M{"$exists": false}}, options.FindOne().SetSort(mongostore.GuestQueueSort())).Decode(&next) == nil && !core.CanOfferInvitation(latest, next, time.Now().UTC()) {
			v.Phase = "waiting"
		}
	}
	if inFlight > 0 {
		v.Phase = "sending"
	}
	return v
}

func (s *server) invitationWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.invitationStep(ctx)
		}
	}
}

func (s *server) invitationStep(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 110*time.Second)
	defer cancel()
	cur, err := s.db.Collection("events").Find(ctx, bson.M{"invitationsEnabled": true, "paymentStatus": "paid", "isDemo": bson.M{"$ne": true}, "publishedAt": bson.M{"$ne": nil}, "startAt": bson.M{"$gt": time.Now().UTC()}}, options.Find().SetSort(bson.D{{Key: "invitationCheckedAt", Value: 1}}).SetLimit(20))
	if err != nil {
		return
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var e core.Event
		if cur.Decode(&e) == nil {
			_, _ = s.db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"invitationCheckedAt": time.Now().UTC()}})
			s.dispatchInvitations(ctx, e)
		}
	}
}
