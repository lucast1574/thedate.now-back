package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type wazendEvent struct {
	ID      string `json:"id"`
	Session string `json:"session"`
	Event   string `json:"event"`
	Payload struct {
		EventCreationKey struct {
			ID string `json:"id"`
		} `json:"eventCreationKey"`
		EventResponse struct {
			Response string `json:"response"`
		} `json:"eventResponse"`
	} `json:"payload"`
}

func validWazendSignature(body []byte, signature, secret string) bool {
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha512="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}

func (s *server) wazendWebhook(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("WAZEND_WEBHOOK_HMAC")
	if len(secret) < 32 {
		bad(w, 503, "Webhook is not configured")
		return
	}
	stamp, err := strconv.ParseInt(r.Header.Get("X-Webhook-Timestamp"), 10, 64)
	if err != nil || time.Since(time.UnixMilli(stamp)) > 5*time.Minute || time.Until(time.UnixMilli(stamp)) > 5*time.Minute {
		bad(w, 401, "Invalid timestamp")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		bad(w, 400, "Invalid body")
		return
	}
	if !validWazendSignature(body, r.Header.Get("X-Webhook-Hmac"), secret) {
		bad(w, 401, "Invalid signature")
		return
	}
	var event wazendEvent
	if json.Unmarshal(body, &event) != nil || event.ID == "" {
		bad(w, 400, "Invalid event")
		return
	}
	if event.Session != os.Getenv("WAZEND_SESSION") || event.Event != "event.response" {
		reply(w, 200, map[string]string{"status": "ignored"})
		return
	}
	response := map[string]string{"GOING": "going", "NOT_GOING": "not_going", "MAYBE": "maybe"}[event.Payload.EventResponse.Response]
	if response == "" || event.Payload.EventCreationKey.ID == "" {
		bad(w, 400, "Invalid event response")
		return
	}
	var g core.Guest
	if err := s.db.Collection("guests").FindOne(r.Context(), bson.M{"invitationMessageId": event.Payload.EventCreationKey.ID}).Decode(&g); err != nil {
		reply(w, 200, map[string]string{"status": "unmatched"})
		return
	}
	var e core.Event
	if err := s.db.Collection("events").FindOne(r.Context(), bson.M{"_id": g.EventID, "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}}).Decode(&e); err != nil {
		reply(w, 200, map[string]string{"status": "ignored"})
		return
	}
	_, err = s.db.Collection("webhookEvents").InsertOne(r.Context(), bson.M{"_id": event.ID, "createdAt": time.Now().UTC()})
	if mongo.IsDuplicateKeyError(err) {
		reply(w, 200, map[string]string{"status": "duplicate"})
		return
	}
	if err != nil {
		bad(w, 500, "Could not record event")
		return
	}
	if err := s.applyResponse(r.Context(), e, g, response, ""); err != nil {
		_, _ = s.db.Collection("webhookEvents").DeleteOne(r.Context(), bson.M{"_id": event.ID})
		if err == errCapacity {
			bad(w, 409, "Event capacity reached")
		} else {
			bad(w, 500, "Could not save response")
		}
		return
	}
	if response == "maybe" {
		go requestMaybeReason(e, g)
	}
	reply(w, 200, map[string]string{"status": "processed"})
}

func requestMaybeReason(e core.Event, g core.Guest) {
	if !wazendReady() {
		return
	}
	base, err := url.Parse(os.Getenv("WAZEND_BASE_URL"))
	if err != nil || base.Scheme != "https" {
		return
	}
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	message := map[string]string{"session": os.Getenv("WAZEND_SESSION"), "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "text": "Reservamos tus cupos temporalmente. Cuéntanos por qué estás en espera desde tu enlace: https://" + host + "/rsvp/" + g.InviteToken}
	body, _ := json.Marshal(message)
	req, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(base.String(), "/")+"/api/sendText", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", os.Getenv("WAZEND_API_KEY"))
	resp, err := wazendClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
