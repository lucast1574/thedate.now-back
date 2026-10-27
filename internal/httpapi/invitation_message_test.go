package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"strings"
	"testing"
)

func TestCustomWhatsAppCopyUsesEachGuestsPersonalLink(t *testing.T) {
	for _, kind := range []string{"general", "wedding"} {
		e := core.Event{Kind: kind, Slug: "celebracion", Title: "Nuestra celebración", Description: "Only in the design", InvitationMessage: "Te esperamos. Confirma en {{enlace}} y mira los detalles."}
		g := core.Guest{Name: "Ana", Phone: "+51999111222", Seats: 2, InviteToken: strings.Repeat("a", 48)}
		data := eventMessage(e, g)["event"].(map[string]any)
		text := data["description"].(string)
		host, _ := core.InvitationHost(kind, e.Slug)
		if !strings.Contains(text, "https://"+host+"/rsvp/"+g.InviteToken) || strings.Contains(text, "{{enlace}}") || strings.Contains(text, e.Description) {
			t.Fatal("custom copy or personal link lost")
		}
	}
}
