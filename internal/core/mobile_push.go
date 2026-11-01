package core

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"time"
)

type MobileDevice struct {
	ID           string    `bson:"_id"`
	UserID       string    `bson:"userId"`
	FID          string    `bson:"fid"`
	Platform     string    `bson:"platform"`
	TokenVersion int       `bson:"tokenVersion"`
	RegisteredAt time.Time `bson:"registeredAt"`
	SeenAt       time.Time `bson:"seenAt"`
	Binding      string    `bson:"binding"`
}

type MobileChange struct {
	ID, EventID, Screen string
	At                  time.Time
}

// No personal names or invitation tokens leave the API in notification payloads.
func MobileChanges(e Event, since time.Time) []MobileChange {
	changes := []MobileChange{}
	add := func(kind, id, screen string, at time.Time) {
		if at.After(since) {
			sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%s", e.ID, kind, id, at.UTC().Format(time.RFC3339Nano))))
			changes = append(changes, MobileChange{ID: fmt.Sprintf("%x", sum), EventID: e.ID, Screen: screen, At: at})
		}
	}
	for id, response := range e.Responses {
		add("response", id, "guests", response.RespondedAt)
	}
	for _, gift := range e.Wishlist.Items {
		if gift.GuestID != "" && gift.ReservedAt != nil {
			add("gift", gift.ID, "gifts", *gift.ReservedAt)
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].At.Equal(changes[j].At) {
			return changes[i].ID < changes[j].ID
		}
		return changes[i].At.Before(changes[j].At)
	})
	return changes
}

func CanReceiveMobilePush(u User, d MobileDevice, e Event) bool {
	if u.ID != d.UserID || u.TokenVersion != d.TokenVersion || e.IsDemo || e.PaymentStatus != "paid" {
		return false
	}
	if e.OwnerID == u.ID {
		return true
	}
	for _, id := range e.CoupleUserIDs {
		if id == u.ID {
			return true
		}
	}
	return false
}
