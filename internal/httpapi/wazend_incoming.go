package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"net/http"
	"strings"
	"time"
)

type wazendIncoming struct {
	ID      string `json:"id"`
	Session string `json:"session"`
	Event   string `json:"event"`
	Payload struct {
		ID        string  `json:"id"`
		From      string  `json:"from"`
		FromMe    bool    `json:"fromMe"`
		Body      string  `json:"body"`
		Timestamp float64 `json:"timestamp"`
		HasMedia  bool    `json:"hasMedia"`
		Media     *struct {
			URL      string `json:"url"`
			MIME     string `json:"mimetype"`
			Filename string `json:"filename"`
		} `json:"media"`
		ReplyTo *struct {
			ID string `json:"id"`
		} `json:"replyTo"`
	} `json:"payload"`
}

func messageIdentity(session, id string) string {
	v := sha256.Sum256([]byte(session + ":" + wazendMessageID(id)))
	return hex.EncodeToString(v[:])
}
func incomingMessage(e wazendIncoming, now time.Time) (core.InboxMessage, bool) {
	p := e.Payload
	phone, ok := strings.CutSuffix(p.From, "@c.us")
	if !ok {
		phone, ok = strings.CutSuffix(p.From, "@s.whatsapp.net")
	}
	if !ok || p.FromMe || !phonePattern.MatchString("+"+phone) || p.ID == "" || len(p.ID) > 512 || (p.Body == "" && !p.HasMedia) {
		return core.InboxMessage{}, false
	}
	created := time.Unix(int64(p.Timestamp), 0).UTC()
	if p.Timestamp <= 0 || created.After(now.Add(5*time.Minute)) {
		created = now
	}
	m := core.InboxMessage{ID: messageIdentity(e.Session, p.ID), ProviderID: p.ID, Session: e.Session, Phone: "+" + phone, Direction: "in", Text: p.Body, CreatedAt: created, ReceivedAt: now, Delivery: "received"}
	if len(m.Text) > 20000 {
		m.Text = m.Text[:20000]
	}
	if p.ReplyTo != nil {
		m.ReplyTo = wazendMessageID(p.ReplyTo.ID)
	}
	if p.HasMedia {
		m.Media = &core.MessageMedia{}
		if p.Media != nil {
			m.Media.URL = p.Media.URL
			m.Media.MIME = p.Media.MIME
			m.Media.Filename = p.Media.Filename
		}
	}
	return m, true
}

func (s *server) receiveWazendMessage(w http.ResponseWriter, r *http.Request, body []byte) {
	var event wazendIncoming
	if json.Unmarshal(body, &event) != nil {
		bad(w, 400, "Invalid message")
		return
	}
	peers := s.messagePeers
	if len(peers) == 0 {
		peers = []*server{s}
	}
	trusted := []*server{}
	var source wazendSettings
	for _, peer := range peers {
		kind := peer.product
		kinds := []string{kind}
		if kind == "" {
			kinds = []string{"general", "wedding"}
		}
		for _, value := range kinds {
			config := wazendFor(value)
			if event.Session == config.Session && len(config.HMAC) >= 32 && validWazendSignature(body, r.Header.Get("X-Webhook-Hmac"), config.HMAC) {
				trusted = append(trusted, peer)
				source = config
				break
			}
		}
	}
	if len(trusted) == 0 {
		bad(w, 401, "Wrong product session")
		return
	}
	if !event.Payload.FromMe {
		resolved, err := resolveMessageSender(r.Context(), source, event.Payload.From)
		if err != nil {
			bad(w, 503, "Could not identify sender; webhook can be retried")
			return
		}
		event.Payload.From = resolved
	}
	m, ok := incomingMessage(event, time.Now().UTC())
	if !ok {
		reply(w, 200, map[string]string{"status": "ignored"})
		return
	}
	if err := routeIncoming(r.Context(), trusted, m); err != nil {
		bad(w, 500, "Could not store message")
		return
	}
	reply(w, 200, map[string]string{"status": "processed"})
}
