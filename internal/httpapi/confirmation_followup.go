package httpapi

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *server) confirmationFollowupStep(ctx context.Context, eventID ...string) {
	s.confirmationKindStep(ctx, "companions", eventID...)
	s.giftFollowupStep(ctx, eventID...)
	s.confirmationKindStep(ctx, "payment", eventID...)
}

func (s *server) confirmationKindStep(parent context.Context, kind string, eventID ...string) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	filter := bson.M{"paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}, "isDemo": bson.M{"$ne": true}, "confirmationFollowups." + kind: bson.M{"$exists": true}}
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
		if cur.Decode(&e) != nil || e.DeletionRequested || core.EventClosed(e, time.Now().UTC()) {
			continue
		}
		config := wazendFor(e.Kind)
		base, err := url.Parse(config.Base)
		if !config.ready() || err != nil || base.Scheme != "https" || base.Host == "" {
			continue
		}
		for id, state := range e.ConfirmationFollowups[kind] {
			if state != "pending" || e.Responses[id].Choice != "going" {
				continue
			}
			var g core.Guest
			if s.db.Collection("guests").FindOne(ctx, bson.M{"_id": id, "eventId": e.ID, "sendState": bson.M{"$ne": "removing"}}).Decode(&g) != nil || g.Phone == "" || g.InviteToken == "" {
				continue
			}
			s.processConfirmationFollowup(ctx, config, kind, e.ID, g)
		}
	}
}

func (s *server) processConfirmationFollowup(ctx context.Context, config wazendSettings, kind, eventID string, g core.Guest) {
	repo := mongostore.Responses{DB: s.db}
	current, err := repo.Snapshot(ctx, eventID)
	if err != nil || current.RevokedGuestTokens[g.InviteToken] || current.DeletionRequested || core.EventClosed(current, time.Now().UTC()) {
		return
	}
	claimed, err := repo.ClaimConfirmationFollowup(ctx, current, g, kind)
	if err != nil || !claimed {
		return
	}
	outcome := "pending"
	latest, err := repo.Snapshot(ctx, eventID)
	if err == nil && latest.Responses[g.ID].Choice == "going" && !latest.RevokedGuestTokens[g.InviteToken] && !latest.DeletionRequested && !core.EventClosed(latest, time.Now().UTC()) {
		if text := confirmationText(latest, g, kind); text != "" {
			payload := map[string]any{"session": config.Session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "text": text, "linkPreview": false}
			outcome = s.sendAutomaticMessage(ctx, latest, g, config, kind, payload)
		} else {
			outcome = "skipped"
		}
	}
	final, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = repo.FinishConfirmationFollowup(final, eventID, g.ID, kind, outcome)
}

func confirmationText(e core.Event, g core.Guest, kind string) string {
	title := core.InvitationTitle(e.Kind, e.Title, e.Celebration)
	prefix := invitationBrand(e.Kind) + " · "
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	link := "https://" + host + core.GuestInvitationPath(g) + "/respuesta"
	if kind == "companions" {
		if g.Seats < 2 || e.Responses[g.ID].PartyRegistered {
			return ""
		}
		return prefix + "¡Gracias por confirmar tu asistencia a " + title + "! Indícanos quiénes te acompañarán o si vas por tu cuenta: " + link
	}
	a := e.Admission
	if e.Kind != "general" || a == nil || !a.Enabled {
		return ""
	}
	balance := core.GuestPaymentBalance(e, g)
	if balance == 0 {
		return ""
	}
	amount := func(cents int64) string {
		return fmt.Sprintf("%d.%02d %s", cents/100, cents%100, a.Currency)
	}
	text := "El saldo de tu invitación para " + title + " es de " + amount(balance) + "."
	if a.Basis == "person" && g.Seats > 1 && !e.Responses[g.ID].PartyRegistered {
		text = "La entrada para " + title + " cuesta " + amount(a.AmountCents) + " por persona. Completa quiénes asistirán antes de pagar: " + link
	}
	return prefix + text + "\n\nCuando hagas el pago, toca Responder en este mensaje y adjunta tu comprobante. ¡Gracias!"
}
