package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *server) conversationHistory(w http.ResponseWriter, r *http.Request) {
	e, err := s.inboxEvent(r)
	if err != nil || e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 404, "Evento no encontrado")
		return
	}
	var guest core.Guest
	if s.db.Collection("guests").FindOne(r.Context(), bson.M{"_id": r.PathValue("guestId"), "eventId": e.ID}).Decode(&guest) != nil || guest.SentAt == nil {
		bad(w, 400, "Primero envía la invitación de este invitado.")
		return
	}
	config := wazendFor(e.Kind)
	base, err := url.Parse(config.Base)
	if err != nil || base.Scheme != "https" || !config.ready() {
		bad(w, 503, "WhatsApp no está disponible.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	endpoint := strings.TrimSuffix(config.Base, "/") + "/api/" + url.PathEscape(config.Session) + "/chats/" + url.PathEscape(strings.TrimPrefix(guest.Phone, "+")+"@c.us") + "/messages?limit=50&downloadMedia=false"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		bad(w, 502, "No pudimos consultar WhatsApp.")
		return
	}
	req.Header.Set("X-Api-Key", config.Key)
	response, err := wazendClient.Do(req)
	if err != nil {
		bad(w, 502, "No pudimos consultar WhatsApp.")
		return
	}
	defer response.Body.Close()
	var raw []json.RawMessage
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&raw) != nil {
		bad(w, 502, "No pudimos recuperar el historial de WhatsApp.")
		return
	}
	peers := s.messagePeers
	if len(peers) == 0 {
		peers = []*server{s}
	}
	trusted := []*server{}
	for _, peer := range peers {
		kind := peer.product
		if kind == "" {
			kind = e.Kind
		}
		c := wazendFor(kind)
		if c.Base == config.Base && c.Session == config.Session {
			trusted = append(trusted, peer)
		}
	}
	filter := bson.M{"eventId": e.ID, "guestId": guest.ID}
	before, err := s.db.Collection("inboxMessages").CountDocuments(ctx, filter)
	if err != nil {
		bad(w, 500, "No pudimos cargar los mensajes.")
		return
	}
	for _, body := range raw {
		var event wazendIncoming
		event.Session = config.Session
		if json.Unmarshal(body, &event.Payload) != nil || event.Payload.FromMe {
			continue
		}
		sender, err := resolveMessageSender(ctx, config, event.Payload.From)
		if err != nil {
			continue
		}
		event.Payload.From = sender
		m, ok := incomingMessage(event, time.Now().UTC())
		if !ok || event.Payload.Timestamp <= 0 || m.Phone != guest.Phone || m.CreatedAt.Before(guest.SentAt.Add(-time.Minute)) {
			continue
		}
		if !candidateMatches(ctx, messageCandidate{s, e, guest}, m) {
			if m.ReplyTo != "" {
				continue
			}
			candidates, lookupErr := messageCandidates(ctx, trusted, m)
			if lookupErr != nil {
				bad(w, 503, "No pudimos identificar el evento de los mensajes.")
				return
			}
			if len(candidates) != 1 || candidates[0].Event.ID != e.ID {
				continue
			}
		}
		if err = routeIncoming(ctx, trusted, m, true); err != nil {
			bad(w, 502, "No pudimos completar la recuperación. Puedes volver a intentarlo sin duplicar mensajes.")
			return
		}
	}
	after, err := s.db.Collection("inboxMessages").CountDocuments(ctx, filter)
	if err != nil {
		bad(w, 500, "No pudimos contar los mensajes.")
		return
	}
	reply(w, 200, map[string]any{"added": after - before, "examined": len(raw)})
}
