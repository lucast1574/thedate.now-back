package core

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type Gift struct {
	ID          string     `bson:"id" json:"id"`
	Title       string     `bson:"title" json:"title"`
	URL         string     `bson:"url" json:"url"`
	ImageURL    string     `bson:"imageUrl,omitempty" json:"imageUrl,omitempty"`
	Description string     `bson:"description,omitempty" json:"description,omitempty"`
	GuestID     string     `bson:"guestId,omitempty" json:"guestId,omitempty"`
	ReservedAt  *time.Time `bson:"reservedAt,omitempty" json:"reservedAt,omitempty"`
	Delivered   bool       `bson:"delivered" json:"delivered"`
}
type Wishlist struct {
	Version int64  `bson:"version" json:"version"`
	Enabled bool   `bson:"enabled" json:"enabled"`
	Items   []Gift `bson:"items" json:"items"`
}

var (
	ErrGiftUnavailable = errors.New("Este regalo ya fue elegido. Escoge otro disponible.")
	ErrGiftAssigned    = errors.New("Ya elegiste un regalo. Cancela tu elección antes de cambiarlo.")
	ErrGiftDisabled    = errors.New("La lista de regalos no está disponible.")
	ErrGiftMissing     = errors.New("No encontramos este regalo.")
	ErrGiftDelivered   = errors.New("El regalo ya fue marcado como recibido. Contacta al organizador.")
)
var giftID = regexp.MustCompile(`^[a-zA-Z0-9-]{1,64}$`)

func ValidWishlist(list Wishlist) bool {
	if list.Version < 0 || len(list.Items) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, item := range list.Items {
		if !giftID.MatchString(item.ID) || seen[item.ID] || len(strings.TrimSpace(item.Title)) < 2 || len(item.Title) > 240 || len(item.Description) > 1000 || len(item.URL) > 2048 || !ValidHTTPSURL(item.URL) || len(item.ImageURL) > 2048 || (item.ImageURL != "" && !ValidHTTPSURL(item.ImageURL)) {
			return false
		}
		seen[item.ID] = true
	}
	return true
}

// Editing presentation never accepts a client-supplied guest assignment.
func EditWishlist(old Wishlist, in Wishlist) (Wishlist, error) {
	if !ValidWishlist(in) {
		return old, errors.New("Revisa los enlaces y nombres de los regalos (máximo 100).")
	}
	existing := map[string]Gift{}
	for _, item := range old.Items {
		existing[item.ID] = item
	}
	next := Wishlist{Version: old.Version + 1, Enabled: len(in.Items) > 0, Items: make([]Gift, 0, len(in.Items))}
	kept := map[string]bool{}
	for _, item := range in.Items {
		prior := existing[item.ID]
		item.GuestID = prior.GuestID
		item.ReservedAt = prior.ReservedAt
		item.Delivered = prior.Delivered
		// A guest's commitment must keep its original product until explicitly released.
		if prior.GuestID != "" && (item.URL != prior.URL || item.Title != prior.Title) {
			return old, errors.New("Libera la reserva antes de cambiar el producto elegido.")
		}
		kept[item.ID] = true
		next.Items = append(next.Items, item)
	}
	for _, item := range old.Items {
		if item.GuestID != "" && !kept[item.ID] {
			return old, errors.New("Libera la reserva antes de eliminar este regalo.")
		}
	}
	return next, nil
}
func DecideGift(e Event, g Guest, id string, release bool, now time.Time) (Wishlist, error) {
	old := e.Wishlist
	if e.RevokedGuestTokens[g.InviteToken] || e.PaymentStatus != "paid" || e.PublishedAt == nil || e.IsDemo || g.EventID != e.ID || g.ID == "" {
		return old, ErrGiftDisabled
	}
	next := old
	next.Items = append([]Gift(nil), old.Items...)
	next.Version++
	index := -1
	for i, item := range next.Items {
		if item.ID == id {
			index = i
		}
		if !release && item.GuestID == g.ID && item.ID != id {
			return old, ErrGiftAssigned
		}
	}
	if index < 0 {
		return old, ErrGiftMissing
	}
	item := &next.Items[index]
	if release {
		if item.GuestID == "" {
			return next, nil
		}
		if item.GuestID != g.ID {
			return old, ErrGiftUnavailable
		}
		if item.Delivered {
			return old, ErrGiftDelivered
		}
		item.GuestID = ""
		item.ReservedAt = nil
		item.Delivered = false
	} else {
		if item.GuestID != "" && item.GuestID != g.ID {
			return old, ErrGiftUnavailable
		}
		if item.GuestID == "" {
			item.GuestID = g.ID
			item.ReservedAt = &now
		}
	}
	return next, nil
}
