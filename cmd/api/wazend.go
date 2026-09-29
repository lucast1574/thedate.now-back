package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var wazendClient = &http.Client{Timeout: 20 * time.Second}

func wazendReady() bool {
	return os.Getenv("WAZEND_BASE_URL") != "" && os.Getenv("WAZEND_API_KEY") != "" && os.Getenv("WAZEND_SESSION") != ""
}

func eventMessage(e core.Event, g core.Guest) map[string]any {
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	link := "https://" + host + "/rsvp/" + g.InviteToken
	return map[string]any{
		"chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us",
		"event": map[string]any{
			"name":               e.Title,
			"description":        "Hola " + g.Name + ", te invitamos a " + e.Title + ". Todos los detalles y tu respuesta: " + link,
			"startTime":          e.StartAt.UTC().Format(time.RFC3339),
			"endTime":            e.StartAt.Add(4 * time.Hour).UTC().Format(time.RFC3339),
			"location":           map[string]string{"name": e.Location},
			"extraGuestsAllowed": false,
		},
	}
}

func extractMessageID(raw []byte) string {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	if id, ok := obj["id"].(string); ok {
		return id
	}
	if id, ok := obj["id"].(map[string]any); ok {
		if serial, ok := id["_serialized"].(string); ok {
			return serial
		}
		if value, ok := id["id"].(string); ok {
			return value
		}
	}
	if key, ok := obj["key"].(map[string]any); ok {
		if id, ok := key["id"].(string); ok {
			return id
		}
	}
	return ""
}

func (s *server) sendInvitations(w http.ResponseWriter, r *http.Request) {
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.PaymentStatus != "paid" || e.PublishedAt == nil {
		bad(w, 403, "Publish the paid event before sending invitations")
		return
	}
	if !wazendReady() {
		bad(w, 503, "Wazend is not configured yet")
		return
	}
	base, err := url.Parse(os.Getenv("WAZEND_BASE_URL"))
	if err != nil || base.Scheme != "https" || base.Host == "" {
		bad(w, 503, "Invalid Wazend URL")
		return
	}
	endpoint := strings.TrimSuffix(base.String(), "/") + "/api/" + url.PathEscape(os.Getenv("WAZEND_SESSION")) + "/events"
	cur, err := s.db.Collection("guests").Find(r.Context(), bson.M{"eventId": e.ID, "sentAt": nil})
	if err != nil {
		bad(w, 500, "Could not load guests")
		return
	}
	defer cur.Close(r.Context())
	var guests []core.Guest
	if err = cur.All(r.Context(), &guests); err != nil {
		bad(w, 500, "Could not load guests")
		return
	}
	sent, failed := 0, 0
	for i, g := range guests {
		if i >= 50 {
			break
		}
		body, _ := json.Marshal(eventMessage(e, g))
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			failed++
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", os.Getenv("WAZEND_API_KEY"))
		resp, err := wazendClient.Do(req)
		if err != nil {
			failed++
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			failed++
			continue
		}
		now := time.Now().UTC()
		messageID := extractMessageID(data)
		_, err = s.db.Collection("guests").UpdateOne(r.Context(), bson.M{"_id": g.ID, "sentAt": nil}, bson.M{"$set": bson.M{"sentAt": now, "invitationMessageId": messageID}})
		if err != nil {
			failed++
			continue
		}
		sent++
	}
	reply(w, 200, map[string]int{"sent": sent, "failed": failed, "remaining": max(0, len(guests)-50)})
}
