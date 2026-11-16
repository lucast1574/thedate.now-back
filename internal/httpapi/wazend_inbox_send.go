package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func sendInboxText(ctx context.Context, config wazendSettings, phone, text, replyTo string) (string, error) {
	base, err := url.Parse(config.Base)
	if err != nil || base.Scheme != "https" || base.Host == "" || !config.ready() {
		return "", errors.New("WhatsApp no está disponible.")
	}
	payload := map[string]any{"session": config.Session, "chatId": strings.TrimPrefix(phone, "+") + "@c.us", "text": text}
	if replyTo != "" {
		payload["reply_to"] = replyTo
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(config.Base, "/")+"/api/sendText", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Api-Key", config.Key)
	req.Header.Set("Content-Type", "application/json")
	response, err := wazendClient.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", errors.New("No pudimos confirmar el envío con WhatsApp.")
	}
	id := extractMessageID(raw)
	if id == "" {
		return "", errors.New("WhatsApp no confirmó el identificador del mensaje.")
	}
	return id, nil
}
func (s *server) replyConversation(w http.ResponseWriter, r *http.Request) {
	e, err := s.inboxEvent(r)
	if err != nil || e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 404, "Evento no encontrado")
		return
	}
	var g core.Guest
	if s.db.Collection("guests").FindOne(r.Context(), bson.M{"_id": r.PathValue("guestId"), "eventId": e.ID}).Decode(&g) != nil {
		bad(w, 404, "Invitado no encontrado")
		return
	}
	var in struct {
		RequestID string `json:"requestId"`
		Text      string `json:"text"`
		ReplyTo   string `json:"replyTo"`
	}
	if decode(r, &in) != nil || uuid.Validate(in.RequestID) != nil || len(strings.TrimSpace(in.Text)) == 0 || len(in.Text) > 4000 {
		bad(w, 400, "Escribe un mensaje de hasta 4000 caracteres.")
		return
	}
	config := wazendFor(e.Kind)
	if !config.ready() {
		bad(w, 503, "WhatsApp no está disponible.")
		return
	}
	u, _ := s.user(r)
	now := time.Now().UTC()
	m := core.InboxMessage{ID: messageIdentity(e.ID, in.RequestID), EventID: e.ID, GuestID: g.ID, Session: config.Session, Phone: g.Phone, Direction: "out", Text: strings.TrimSpace(in.Text), CreatedAt: now, ReceivedAt: now, SentBy: u.ID, Delivery: "sending"}
	replyTo := ""
	if in.ReplyTo != "" {
		original, err := (mongostore.Inbox{DB: s.db}).Message(r.Context(), in.ReplyTo)
		if err != nil || original.EventID != e.ID || original.GuestID != g.ID {
			bad(w, 400, "Mensaje original no encontrado")
			return
		}
		replyTo = original.ProviderID
	}
	repo := mongostore.Inbox{DB: s.db}
	inserted, err := repo.Put(r.Context(), m)
	if err != nil {
		bad(w, 500, "No pudimos preparar el envío.")
		return
	}
	if !inserted {
		existing, err := repo.Message(r.Context(), m.ID)
		if err != nil || existing.Text != m.Text || existing.GuestID != m.GuestID {
			bad(w, 409, "Este envío ya está registrado con otro contenido.")
			return
		}
		reply(w, 200, existing)
		return
	}
	id, sendErr := sendInboxText(r.Context(), config, g.Phone, "*"+invitationBrand(e.Kind)+" · "+e.Title+"*\n"+m.Text, replyTo)
	m.ProviderID = id
	m.Delivery = "sent"
	if sendErr != nil {
		m.Delivery = "uncertain"
	}
	// Finish under a fresh context even if the browser disconnected after sending.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.db.Collection("inboxMessages").UpdateOne(ctx, bson.M{"_id": m.ID, "delivery": "sending"}, bson.M{"$set": bson.M{"providerId": id, "delivery": m.Delivery}})
	if err != nil {
		bad(w, 502, "El envío requiere verificación. No lo repitas para evitar duplicados.")
		return
	}
	reply(w, 200, m)
}
