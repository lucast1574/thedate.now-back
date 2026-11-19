package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/url"
	"strings"
	"time"
)

// The durable queue is written with RSVP. Claim before the provider call, and
// retain uncertain outcomes rather than risk sending a duplicate personal link.
func (s *server) giftFollowupStep(parent context.Context, eventID ...string) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	filter := bson.M{"paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}, "isDemo": bson.M{"$ne": true}, "giftFollowups": bson.M{"$exists": true}}
	if len(eventID) > 0 {
		filter["_id"] = eventID[0]
	}
	cur, err := s.db.Collection("events").Find(ctx, filter)
	if err != nil {
		return
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var e core.Event
		if cur.Decode(&e) != nil || core.EventClosed(e, time.Now().UTC()) {
			continue
		}
		config := wazendFor(e.Kind)
		base, err := url.Parse(config.Base)
		if !config.ready() || err != nil || base.Scheme != "https" || base.Host == "" {
			continue
		}
		for id, state := range e.GiftFollowups {
			if !giftFollowupEligible(e, id, state) {
				continue
			}
			var g core.Guest
			if s.db.Collection("guests").FindOne(ctx, bson.M{"_id": id, "eventId": e.ID, "sendState": bson.M{"$ne": "removing"}}).Decode(&g) != nil || g.Phone == "" {
				continue
			}
			if !core.ConfirmationFollowupNeeded(e, g) && state == "pending" {
				continue
			}
			repo := mongostore.Responses{DB: s.db}
			claimed, err := repo.ClaimGiftFollowup(ctx, e, g, time.Now().UTC())
			if err != nil || !claimed {
				continue
			}
			sendGiftFollowup(ctx, repo, e.ID, g, state, config)
		}
	}
}

func giftFollowupEligible(e core.Event, id, state string) bool {
	if e.Responses[id].Choice != "going" {
		return false
	}
	return state == "pending" || state == "pending_empty" || (state == "sent_empty" && core.GiftListAvailable(e))
}

func sendGiftFollowup(ctx context.Context, repo mongostore.Responses, eventID string, g core.Guest, state string, config wazendSettings) {
	finish := repo.FinishGiftFollowup
	if state == "sent_empty" {
		finish = repo.FinishGiftReadyNotice
	}
	latest, err := repo.Snapshot(ctx, eventID)
	if err != nil || !giftFollowupEligible(latest, g.ID, state) || latest.RevokedGuestTokens[g.InviteToken] || latest.DeletionRequested || core.EventClosed(latest, time.Now().UTC()) {
		_ = finish(ctx, eventID, g.ID, state)
		return
	}
	allowEmpty := state == "pending_empty"
	includeGifts := core.GiftListAvailable(latest) || (allowEmpty && core.GiftsEnabled(latest))
	includeCompanions := g.Seats > 1 && !latest.Responses[g.ID].PartyRegistered
	if !includeGifts && !includeCompanions {
		_ = finish(ctx, eventID, g.ID, state)
		return
	}
	message := giftConfirmationMessage(latest, g, config.Session, includeGifts)
	if state == "sent_empty" {
		message = giftReadyMessage(latest, g, config.Session)
	}
	_, outcome := postInvitationMessage(ctx, strings.TrimSuffix(config.Base, "/")+"/api/sendText", config.Key, message)
	if state == "sent_empty" {
		outcome = "ready_" + outcome
	} else if outcome == "sent" && includeGifts && len(latest.Wishlist.Items) == 0 && !latest.Wishlist.WasPopulated {
		outcome = "sent_empty"
	}
	finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = finish(finalCtx, eventID, g.ID, outcome)
}

func giftConfirmationMessage(e core.Event, g core.Guest, session string, includeGifts bool) map[string]any {
	e.Title = core.InvitationTitle(e.Kind, e.Title, e.Celebration)
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	link := "https://" + host + core.GuestInvitationPath(g) + "/regalos"
	text := invitationBrand(e.Kind) + " · Gracias por confirmar tu asistencia a " + e.Title + "."
	if includeGifts {
		text += " Si deseas elegir un regalo, aquí tienes tu acceso personal a la lista, sin iniciar sesión: " + link
	}
	if g.Seats > 1 && !e.Responses[g.ID].PartyRegistered {
		text += "\n\nRegistra también los nombres de tus acompañantes y cuántas personas asistirán: https://" + host + guestInvitationPath(g) + "/respuesta"
	}
	return map[string]any{"session": session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "text": text, "linkPreview": false}
}

func giftReadyMessage(e core.Event, g core.Guest, session string) map[string]any {
	title := core.InvitationTitle(e.Kind, e.Title, e.Celebration)
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	link := "https://" + host + core.GuestInvitationPath(g) + "/regalos"
	text := invitationBrand(e.Kind) + " · ¡La lista de regalos de " + title + " ya tiene regalos! Puedes verla aquí: " + link
	return map[string]any{"session": session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "text": text, "linkPreview": false}
}
