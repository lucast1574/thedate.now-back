package httpapi

import (
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"regexp"
	"time"
)

var inviteTokenPattern = regexp.MustCompile(`^[a-f0-9]{48}$`)

func (s *server) giftInvitation(r *http.Request) (core.Event, core.Guest, error) {
	var e core.Event
	var g core.Guest
	if !inviteTokenPattern.MatchString(r.PathValue("token")) {
		return e, g, core.ErrGiftDisabled
	}
	err := s.db.Collection("guests").FindOne(r.Context(), bson.M{"inviteToken": r.PathValue("token")}).Decode(&g)
	if err != nil {
		return e, g, err
	}
	err = s.db.Collection("events").FindOne(r.Context(), bson.M{"_id": g.EventID, "paymentStatus": "paid", "isDemo": bson.M{"$ne": true}, "publishedAt": bson.M{"$ne": nil}}).Decode(&e)
	if err == nil && e.RevokedGuestTokens[g.InviteToken] {
		err = core.ErrGiftDisabled
	}
	return e, g, err
}
func publicWishlist(e core.Event, g core.Guest) map[string]any {
	e = core.PublishedEvent(e)
	items := []map[string]any{}
	for _, item := range e.Wishlist.Items {
		items = append(items, map[string]any{"id": item.ID, "title": item.Title, "url": item.URL, "description": item.Description, "hasImage": item.ImageURL != "", "reserved": item.GuestID != "", "mine": g.ID != "" && item.GuestID == g.ID, "delivered": g.ID != "" && item.GuestID == g.ID && item.Delivered})
	}
	return map[string]any{"event": map[string]any{"id": e.ID, "kind": e.Kind, "slug": e.Slug, "title": e.Title}, "guestName": core.GuestFullName(g), "items": items, "version": e.Wishlist.Version}
}
func (s *server) publicWishlist(w http.ResponseWriter, r *http.Request) {
	e, g, err := s.giftInvitation(r)
	if err != nil {
		bad(w, 404, "La lista no está disponible o el código de invitado no es válido.")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	reply(w, 200, publicWishlist(e, g))
}
func (s *server) chooseGift(w http.ResponseWriter, r *http.Request) {
	_, g, err := s.giftInvitation(r)
	if err != nil {
		bad(w, 404, "La lista no está disponible.")
		return
	}
	if !s.giftRequestLimit(w, r, "gift-choice:"+g.ID, 30) {
		return
	}
	var in struct {
		GiftID  string `json:"giftId"`
		Release bool   `json:"release"`
	}
	if decode(r, &in) != nil || len(in.GiftID) > 64 || in.GiftID == "" {
		bad(w, 400, "Elige un regalo.")
		return
	}
	service := application.Wishlists{Repository: mongostore.Wishlists{DB: s.db}, Now: func() time.Time { return time.Now().UTC() }}
	err = service.Choose(r.Context(), g, in.GiftID, in.Release)
	if err != nil {
		status := 409
		if errors.Is(err, core.ErrGiftDisabled) {
			status = 404
		}
		bad(w, status, err.Error())
		return
	}
	s.publicWishlist(w, r)
}
