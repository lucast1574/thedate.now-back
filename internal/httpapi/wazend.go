package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var wazendClient = &http.Client{Timeout: 20 * time.Second}

func eventMessage(e core.Event, g core.Guest) map[string]any {
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	link := "https://" + host + "/rsvp/" + g.InviteToken
	return map[string]any{
		"chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us",
		"event": map[string]any{
			"name":               invitationBrand(e.Kind) + " · " + e.Title,
			"description":        invitationBrand(e.Kind) + " · Hola " + g.Name + ", te invitamos a " + e.Title + ". Tu invitación permite hasta " + strconv.Itoa(g.Seats) + " personas (incluyéndote). Confirma y registra a tus acompañantes aquí: " + link,
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
	config := wazendFor(e.Kind)
	if !config.ready() {
		bad(w, 503, "Wazend is not configured yet")
		return
	}
	base, err := url.Parse(config.Base)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		bad(w, 503, "Invalid Wazend URL")
		return
	}
	endpoint := strings.TrimSuffix(base.String(), "/") + "/api/" + url.PathEscape(config.Session) + "/events"
	cur, err := s.db.Collection("guests").Find(r.Context(), bson.M{"eventId": e.ID, "sentAt": nil, "sendClaim": bson.M{"$exists": false}}, options.Find().SetLimit(5))
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
		if i >= 5 {
			break
		}
		claim := uuid.NewString()
		claimed, claimErr := s.db.Collection("guests").UpdateOne(r.Context(), bson.M{"_id": g.ID, "sentAt": nil, "sendClaim": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"sendClaim": claim, "sendState": "sending", "sendStartedAt": time.Now().UTC()}})
		if claimErr != nil || claimed.MatchedCount != 1 {
			continue
		}
		// Persist unknown outcomes; automatic retry could duplicate an external send.
		finish := func(fields bson.M, release bool) bool {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			update := bson.M{"$set": fields}
			if release {
				update["$unset"] = bson.M{"sendClaim": ""}
			}
			result, err := s.db.Collection("guests").UpdateOne(ctx, bson.M{"_id": g.ID, "sendClaim": claim}, update)
			return err == nil && result.MatchedCount == 1
		}
		body, _ := json.Marshal(eventMessage(e, g))
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			finish(bson.M{"sendState": "unknown"}, false)
			failed++
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", config.Key)
		resp, err := wazendClient.Do(req)
		if err != nil {
			finish(bson.M{"sendState": "unknown"}, false)
			failed++
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			finish(bson.M{"sendState": "failed"}, resp.StatusCode >= 400 && resp.StatusCode < 500)
			failed++
			continue
		}
		now := time.Now().UTC()
		messageID := extractMessageID(data)
		if messageID == "" || !finish(bson.M{"sentAt": now, "invitationMessageId": messageID, "sendState": "sent"}, false) {
			finish(bson.M{"sendState": "unknown"}, false)
			failed++
			continue
		}

		sent++
	}
	remaining, _ := s.db.Collection("guests").CountDocuments(r.Context(), bson.M{"eventId": e.ID, "sentAt": nil, "sendClaim": bson.M{"$exists": false}})
	uncertain, _ := s.db.Collection("guests").CountDocuments(r.Context(), bson.M{"eventId": e.ID, "sentAt": nil, "sendClaim": bson.M{"$exists": true}})
	reply(w, 200, map[string]int{"sent": sent, "failed": failed, "remaining": int(remaining), "uncertain": int(uncertain)})
}
