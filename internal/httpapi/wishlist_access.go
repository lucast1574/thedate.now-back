package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var accessPhone = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func (s *server) wishlistAccess(w http.ResponseWriter, r *http.Request) {
	e, err := s.catalogEvent(r)
	if err != nil {
		bad(w, 404, "La lista no está disponible.")
		return
	}
	var in struct {
		Phone  string `json:"phone"`
		GiftID string `json:"giftId"`
	}
	if decode(r, &in) != nil || !accessPhone.MatchString(in.Phone) {
		bad(w, 400, "Introduce tu número con el código del país.")
		return
	}
	if in.GiftID != "" {
		found := false
		for _, item := range e.Wishlist.Items {
			if item.ID == in.GiftID {
				found = true
				break
			}
		}
		if !found {
			bad(w, 400, "Este regalo ya no está en la lista.")
			return
		}
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !s.giftRequestLimit(w, r, "gift-access-ip:"+ip, 60) || !s.giftRequestLimit(w, r, "gift-access-event:"+e.ID, 20) {
		return
	}
	hash := sha256.Sum256([]byte(in.Phone))
	claimed, err := mongostore.ClaimGiftAccess(r.Context(), s.db, e.ID+hex.EncodeToString(hash[:]), time.Now().UTC())
	if err != nil {
		bad(w, 503, "Inténtalo de nuevo en un momento.")
		return
	}
	var g core.Guest
	if claimed && s.db.Collection("guests").FindOne(r.Context(), bson.M{"eventId": e.ID, "phone": in.Phone}).Decode(&g) == nil && !e.RevokedGuestTokens[g.InviteToken] {
		sendGiftAccess(r, e, g, in.GiftID)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	// Never reveal whether a phone belongs to the guest list or return its bearer token.
	reply(w, 202, map[string]string{"status": "requested"})
}
func sendGiftAccess(r *http.Request, e core.Event, g core.Guest, giftID string) {
	config := wazendFor(e.Kind)
	base, err := url.Parse(config.Base)
	if !config.ready() || err != nil || base.Scheme != "https" {
		return
	}
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	suffix := "/regalos"
	if giftID != "" {
		suffix += "?elegir=" + url.QueryEscape(giftID)
	}
	message := map[string]any{"session": config.Session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "text": invitationBrand(e.Kind) + " · Solicitaste tu acceso a la lista de regalos de " + e.Title + ". Elige tu regalo desde aquí: https://" + host + core.GuestInvitationPath(g) + suffix, "linkPreview": false}
	body, _ := json.Marshal(message)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimSuffix(base.String(), "/")+"/api/sendText", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", config.Key)
	res, err := wazendClient.Do(req)
	if err == nil {
		res.Body.Close()
	}
}
