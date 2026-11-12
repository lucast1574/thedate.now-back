package httpapi

import (
	"strings"
	"testing"

	"github.com/lucast1574/thedate.now-back/internal/core"
)

func TestConfirmationWithoutGiftsStillRegistersCompanions(t *testing.T) {
	disabled := false
	for _, list := range []core.Wishlist{{}, {Items: []core.Gift{{ID: "saved-gift"}}}} {
		e := core.Event{Kind: "general", Slug: "celebracion", Title: "Celebración", Celebration: &core.Celebration{GiftsEnabled: &disabled}, Wishlist: list, Responses: map[string]core.Response{"g": {Choice: "going"}}}
		g := core.Guest{ID: "g", Seats: 2, InviteToken: strings.Repeat("a", 48)}
		if !core.ConfirmationFollowupNeeded(e, g) {
			t.Fatal("companion registration lost without gifts")
		}
		text := giftConfirmationMessage(e, g, "test")["text"].(string)
		if strings.Contains(text, "/regalos") || strings.Contains(text, "elegir un regalo") || !strings.Contains(text, "/respuesta") {
			t.Fatal(text)
		}
		g.Seats = 1
		if core.ConfirmationFollowupNeeded(e, g) {
			t.Fatal("unnecessary message for a solo guest without gifts")
		}
	}
}
