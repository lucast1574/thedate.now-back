package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
)

var wazendClient = &http.Client{Timeout: 20 * time.Second}

func eventMessage(e core.Event, g core.Guest) map[string]any {
	e = core.PublishedEvent(e)
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	text := core.InvitationMessage(e.InvitationMessage, "https://"+host+guestInvitationPath(g)) + "\n\n" + invitationBrand(e.Kind) + " · " + e.Title + "\nCódigo: " + core.EventMessageCode(e.ID) + "\nPuedes escribirnos por este mismo WhatsApp."
	if e.Kind == "general" && e.Admission != nil && e.Admission.Enabled {
		a := e.Admission
		basis := "por persona"
		if a.Basis == "group" {
			basis = "por invitación"
		}
		text += fmt.Sprintf("\n\nCosto: %d.%02d %s %s\n%s\nEnvía la foto del comprobante a este mismo WhatsApp.", a.AmountCents/100, a.AmountCents%100, a.Currency, basis, a.Instructions)
	}
	return map[string]any{"session": wazendFor(e.Kind).Session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "text": text, "linkPreview": true, "linkPreviewHighQuality": true}
}
func invitationPoll(e core.Event, g core.Guest) map[string]any {
	options := []string{"Sí", "No"}
	if core.MaybeAllowed(e) {
		options = append(options, "Tal vez")
	}
	return map[string]any{"session": wazendFor(e.Kind).Session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "poll": map[string]any{"name": "¿Nos acompañas?", "options": options, "multipleAnswers": false}}
}

func extractMessageID(raw []byte) string {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	if id, ok := obj["id"].(string); ok {
		return wazendMessageID(id)
	}
	if id, ok := obj["id"].(map[string]any); ok {
		if serial, ok := id["_serialized"].(string); ok {
			return wazendMessageID(serial)
		}
		if value, ok := id["id"].(string); ok {
			return wazendMessageID(value)
		}
	}
	if key, ok := obj["key"].(map[string]any); ok {
		if id, ok := key["id"].(string); ok {
			return wazendMessageID(id)
		}
	}
	return ""
}
