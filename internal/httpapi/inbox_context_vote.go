package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Context polls are handled before attendance polls; selecting an event never
// confirms attendance or records a payment.
func (s *server) receiveContextVote(w http.ResponseWriter, r *http.Request, event wazendEvent, body []byte) bool {
	if event.Event != "poll.vote" || !event.Payload.Poll.FromMe {
		return false
	}
	peers := s.messagePeers
	if len(peers) == 0 {
		peers = []*server{s}
	}
	id := wazendMessageID(event.Payload.Poll.ID)
	for _, coordinator := range peers {
		var m core.InboxMessage
		err := coordinator.db.Collection("inboxMessages").FindOne(r.Context(), bson.M{"session": event.Session, "contextPolls.id": id}).Decode(&m)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			bad(w, 500, "Could not load event selection")
			return true
		}
		configKind := coordinator.product
		if len(m.ContextChoices) > 0 {
			configKind = m.ContextChoices[0].Kind
		}
		config := wazendFor(configKind)
		if config.Session != event.Session || !validWazendSignature(body, r.Header.Get("X-Webhook-Hmac"), config.HMAC) {
			bad(w, 401, "Wrong product session")
			return true
		}
		if event.Payload.Vote.FromMe || len(event.Payload.Vote.SelectedOptions) != 1 {
			reply(w, 200, map[string]string{"status": "ignored"})
			return true
		}
		sender, err := resolveMessageSender(r.Context(), config, event.Payload.Vote.From)
		if err != nil {
			bad(w, 503, "Could not identify voter")
			return true
		}
		phone := strings.TrimPrefix(m.Phone, "+")
		if sender != phone+"@c.us" && sender != phone+"@s.whatsapp.net" {
			reply(w, 200, map[string]string{"status": "ignored"})
			return true
		}
		if m.EventID != "" && !strings.HasPrefix(m.EventID, "resolving:") {
			reply(w, 200, map[string]string{"status": "duplicate"})
			return true
		}
		for _, poll := range m.ContextPolls {
			if poll.ID != id {
				continue
			}
			choices, more := contextPage(m.ContextChoices, poll.Page)
			selected := event.Payload.Vote.SelectedOptions[0]
			if more && selected == moreContextEvents {
				if coordinator.sendContextPage(r.Context(), m, m.ContextChoices, configKind, poll.Page+1) != nil {
					bad(w, 500, "Could not show events")
				} else {
					reply(w, 200, map[string]string{"status": "processed"})
				}
				return true
			}
			candidates, err := messageCandidates(r.Context(), peers, m)
			if err != nil {
				bad(w, 500, "Could not identify event")
				return true
			}
			for _, choice := range choices {
				if choice.Label != selected {
					continue
				}
				for _, c := range candidates {
					if c.Event.ID != choice.EventID || c.Guest.ID != choice.GuestID || c.Event.Kind != choice.Kind {
						continue
					}
					if associatePendingMessage(r.Context(), coordinator, c, m) != nil {
						bad(w, 500, "Could not associate message")
					} else {
						reply(w, 200, map[string]string{"status": "processed"})
					}
					return true
				}
			}
		}
		reply(w, 200, map[string]string{"status": "ignored"})
		return true
	}
	return false
}
