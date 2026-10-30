package core

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestGuestPathKeepsOriginalTokenAndPersonalizesName(t *testing.T) {
	g := Guest{Name: "Lúcás", LastName: "Santillán", InviteToken: strings.Repeat("00", 24)}
	if got := GuestInvitationPath(g); got != "/para/lucas-santillan-"+strings.Repeat("A", 32) {
		t.Fatal(got)
	}
	for _, token := range []string{strings.Repeat("aa", 24), strings.Repeat("0123456789abcdef", 3)} {
		g.InviteToken = token
		path := GuestInvitationPath(g)
		data, err := base64.RawURLEncoding.DecodeString(path[len(path)-32:])
		if err != nil || hex.EncodeToString(data) != token {
			t.Fatal(path)
		}
	}
}
func TestGiftsWorkWithoutLegacyActivationFlag(t *testing.T) {
	e := giftEvent()
	e.Wishlist.Enabled = false
	g := Guest{ID: "guest", EventID: e.ID}
	if _, err := DecideGift(e, g, "a", false, time.Now()); err != nil {
		t.Fatal(err)
	}
	next, err := EditWishlist(Wishlist{}, Wishlist{Items: e.Wishlist.Items})
	if err != nil || !next.Enabled {
		t.Fatal(next, err)
	}
}
