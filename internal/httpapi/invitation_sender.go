package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *server) invitationPageReady(ctx context.Context, e core.Event) bool {
	if e.IsDemo || e.PaymentStatus != "paid" || e.PublishedAt == nil {
		return false
	}
	if s.invitationReady != nil {
		return s.invitationReady(ctx, e)
	}
	d, err := (mongostore.Deployments{DB: s.db}).Get(ctx, e.ID)
	if err != nil || d.Phase != "ready" {
		return false
	}
	client, err := deploymentClientFor(s.product)
	return err == nil && client.PublicPageReady(ctx, e, d)
}

func (s *server) dispatchInvitations(ctx context.Context, e core.Event) {
	config := wazendFor(e.Kind)
	base, err := url.Parse(config.Base)
	if !config.ready() || err != nil || base.Scheme != "https" || base.Host == "" || !wazendSessionReady(ctx, config) {
		return
	}
	ready := s.invitationPageReady(ctx, e)
	if _, err := s.db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"invitationWebReady": ready}}); err != nil || !ready {
		return
	}
	endpoint := strings.TrimSuffix(base.String(), "/") + "/api/sendPoll"
	latest, err := (mongostore.Responses{DB: s.db}).Snapshot(ctx, e.ID)
	if err != nil {
		return
	}
	resolved := bson.A{}
	for id, response := range latest.Responses {
		if response.Choice != "" && response.Choice != "pending" {
			resolved = append(resolved, id)
		}
	}
	cur, err := s.db.Collection("guests").Find(ctx, bson.M{"eventId": e.ID, "_id": bson.M{"$nin": resolved}, "sentAt": nil, "sendClaim": bson.M{"$exists": false}}, options.Find().SetSort(mongostore.GuestQueueSort()).SetLimit(2))
	if err != nil {
		return
	}
	defer cur.Close(ctx)
	var guests []core.Guest
	if cur.All(ctx, &guests) != nil {
		return
	}
	queue := mongostore.InvitationQueue{DB: s.db}
	for _, g := range guests {
		reserved, err := queue.Reserve(ctx, e.ID, g, time.Now().UTC())
		if err != nil || !reserved {
			break
		}
		s.sendOneInvitation(ctx, e, g, endpoint, config.Key)
	}
}

func (s *server) sendOneInvitation(ctx context.Context, e core.Event, g core.Guest, endpoint, key string) {
	claim := uuid.NewString()
	claimed, err := s.db.Collection("guests").UpdateOne(ctx, bson.M{"_id": g.ID, "sentAt": nil, "sendClaim": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"sendClaim": claim, "sendState": "sending", "sendStartedAt": time.Now().UTC()}})
	if err != nil || claimed.MatchedCount != 1 {
		return
	}
	// Unknown outcomes retain the claim and offered seats. Never retry them blindly.
	finish := func(fields bson.M, rejected bool) bool {
		finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result, err := s.db.Collection("guests").UpdateOne(finalCtx, bson.M{"_id": g.ID, "sendClaim": claim}, bson.M{"$set": fields})
		if err == nil && result.MatchedCount == 1 && rejected {
			err = (mongostore.InvitationQueue{DB: s.db}).Release(finalCtx, e.ID, g.ID)
		}
		return err == nil && result.MatchedCount == 1
	}
	config := wazendFor(e.Kind)
	if g.InvitationTextMessageID == "" {
		textID, state := postInvitationMessage(ctx, strings.TrimSuffix(config.Base, "/")+"/api/sendText", key, eventMessage(e, g))
		if state != "sent" {
			finish(bson.M{"sendState": state}, state == "failed")
			return
		}
		if !finish(bson.M{"invitationTextMessageId": textID}, false) {
			return
		}
	}
	messageID, state := postInvitationMessage(ctx, endpoint, key, invitationPoll(e, g))
	if state != "sent" {
		finish(bson.M{"sendState": state}, state == "failed")
		return
	}
	if !finish(bson.M{"sentAt": time.Now().UTC(), "invitationMessageId": messageID, "sendState": "sent"}, false) {
		finish(bson.M{"sendState": "unknown"}, false)
	}
}

// Legacy endpoint uses the same readiness gate, queue and capacity reservations.
func (s *server) sendInvitations(w http.ResponseWriter, r *http.Request) {
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 403, "Pay before sending invitations")
		return
	}
	if !s.invitationPageReady(r.Context(), e) {
		bad(w, 409, "Wait until the invitation website is ready")
		return
	}
	if !wazendFor(e.Kind).ready() {
		bad(w, 503, "WhatsApp is not configured yet")
		return
	}
	s.dispatchInvitations(r.Context(), e)
	remaining, _ := s.db.Collection("guests").CountDocuments(r.Context(), bson.M{"eventId": e.ID, "sentAt": nil, "sendClaim": bson.M{"$exists": false}})
	uncertain, _ := s.db.Collection("guests").CountDocuments(r.Context(), bson.M{"eventId": e.ID, "sentAt": nil, "sendClaim": bson.M{"$exists": true}})
	sent, _ := s.db.Collection("guests").CountDocuments(r.Context(), bson.M{"eventId": e.ID, "sentAt": bson.M{"$ne": nil}})
	failed, _ := s.db.Collection("guests").CountDocuments(r.Context(), bson.M{"eventId": e.ID, "sendState": "failed"})
	reply(w, 200, map[string]int{"sent": int(sent), "failed": int(failed), "remaining": int(remaining), "uncertain": int(uncertain)})
}
