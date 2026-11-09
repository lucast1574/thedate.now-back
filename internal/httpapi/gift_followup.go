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
func (s *server) giftFollowupStep(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	cur, err := s.db.Collection("events").Find(ctx, bson.M{"paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}, "isDemo": bson.M{"$ne": true}, "giftFollowups": bson.M{"$exists": true}})
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
			if state != "pending" || e.Responses[id].Choice != "going" {
				continue
			}
			var g core.Guest
			if s.db.Collection("guests").FindOne(ctx, bson.M{"_id": id, "eventId": e.ID}).Decode(&g) != nil || g.Phone == "" {
				continue
			}
			repo := mongostore.Responses{DB: s.db}
			claimed, err := repo.ClaimGiftFollowup(ctx, e, g, time.Now().UTC())
			if err != nil || !claimed {
				continue
			}
			_, outcome := postInvitationMessage(ctx, strings.TrimSuffix(config.Base, "/")+"/api/sendText", config.Key, giftConfirmationMessage(e, g, config.Session))
			finish, done := context.WithTimeout(context.Background(), 5*time.Second)
			_ = repo.FinishGiftFollowup(finish, e.ID, id, outcome)
			done()
		}
	}
}

func giftConfirmationMessage(e core.Event, g core.Guest, session string) map[string]any {
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	link := "https://" + host + core.GuestInvitationPath(g) + "/regalos"
	text := invitationBrand(e.Kind) + " · Gracias por confirmar tu asistencia a " + e.Title + ". Si deseas elegir un regalo, aquí tienes tu acceso personal a la lista, sin iniciar sesión: " + link
	if g.Seats > 1 && !e.Responses[g.ID].PartyRegistered {
		text += "\n\nRegistra también los nombres de tus acompañantes y cuántas personas asistirán: https://" + host + guestInvitationPath(g) + "/respuesta"
	}
	return map[string]any{"session": session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "text": text, "linkPreview": false}
}
