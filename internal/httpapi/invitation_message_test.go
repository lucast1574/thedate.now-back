package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"strings"
	"testing"
	"time"
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

func TestWhatsAppEventDatesUseUnixSeconds(t *testing.T) {
	start := time.Date(2026, 10, 30, 18, 0, 0, 0, time.FixedZone("Bogota", -5*3600))
	e := core.Event{Kind: "wedding", Slug: "boda", StartAt: start}
	data := eventMessage(e, core.Guest{})["event"].(map[string]any)
	if data["startTime"] != start.Unix() || data["endTime"] != start.Add(4*time.Hour).Unix() {
		t.Fatal("event dates do not match provider contract")
	}
}

func TestWhatsAppMessageIDsNormalizeProviderResponseVariants(t *testing.T) {
	for _, payload := range []string{
		`{"id":"true_51999111222@c.us_3EBABC"}`,
		`{"id":{"_serialized":"true_51999111222@c.us_3EBABC"}}`,
		`{"id":{"id":"3EBABC"}}`, `{"key":{"id":"3EBABC"}}`,
	} {
		if got := extractMessageID([]byte(payload)); got != "3EBABC" {
			t.Fatalf("unexpected message key %q", got)
		}
	}
	if got := wazendMessageID("false_51999888777@c.us_3EBABC"); got != "3EBABC" {
		t.Fatal("response prefix changed the key")
	}
}
