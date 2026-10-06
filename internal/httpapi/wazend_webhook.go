package httpapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
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
	configs := []wazendSettings{wazendFor("wedding"), wazendFor("general")}
	if len(configs[0].HMAC) < 32 && len(configs[1].HMAC) < 32 {
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
	valid := false
	for _, config := range configs {
		if len(config.HMAC) >= 32 && validWazendSignature(body, r.Header.Get("X-Webhook-Hmac"), config.HMAC) {
			valid = true
		}
	}
	if !valid {
		bad(w, 401, "Invalid signature")
		return
	}
	var event wazendEvent
	if json.Unmarshal(body, &event) != nil || event.ID == "" {
		bad(w, 400, "Invalid event")
		return
	}
	if event.Event != "event.response" {
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
	config := wazendFor(e.Kind)
	if event.Session != config.Session || !validWazendSignature(body, r.Header.Get("X-Webhook-Hmac"), config.HMAC) {
		bad(w, 401, "Wrong product session")
		return
	}
	receipt := sha256.Sum256([]byte("wazend:" + event.ID))
	err = s.responses().Apply(r.Context(), e.ID, g, response, "", hex.EncodeToString(receipt[:]))
	if errors.Is(err, core.ErrDuplicate) {
		reply(w, 200, map[string]string{"status": "duplicate"})
		return
	}
	if err != nil {
		if errors.Is(err, errCapacity) {
			bad(w, 409, "Event capacity reached")
		} else {
			bad(w, 500, "Could not save response")
		}
		return
	}
	if response == "maybe" {
		go requestRSVPFollowup(e, g, "Reservamos tus cupos temporalmente. Cuéntanos por qué estás en espera y registra tus acompañantes desde tu enlace: ")
	}
	if response == "going" && g.Seats > 1 {
		go requestRSVPFollowup(e, g, "Gracias por confirmar. Registra los nombres de tus acompañantes y cuántas personas asistirán desde tu enlace: ")
	}
	reply(w, 200, map[string]string{"status": "processed"})
}

func requestRSVPFollowup(e core.Event, g core.Guest, text string) {
	config := wazendFor(e.Kind)
	if !config.ready() {
		return
	}
	base, err := url.Parse(config.Base)
	if err != nil || base.Scheme != "https" {
		return
	}
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	message := map[string]string{"session": config.Session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "text": invitationBrand(e.Kind) + " · " + text + "https://" + host + "/rsvp/" + g.InviteToken}
	body, _ := json.Marshal(message)
	req, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(base.String(), "/")+"/api/sendText", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", config.Key)
	resp, err := wazendClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
