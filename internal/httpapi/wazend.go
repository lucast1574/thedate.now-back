package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
)

var wazendClient = &http.Client{Timeout: 20 * time.Second}

func eventMessage(e core.Event, g core.Guest) map[string]any {
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	return map[string]any{"session": wazendFor(e.Kind).Session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "text": core.InvitationMessage(e.InvitationMessage, "https://"+host+"/rsvp/"+g.InviteToken), "linkPreview": false}
}
func invitationPoll(e core.Event, g core.Guest) map[string]any {
	return map[string]any{"session": wazendFor(e.Kind).Session, "chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us", "poll": map[string]any{"name": "¿Nos acompañas?", "options": []string{"Sí", "No", "Tal vez"}, "multipleAnswers": false}}
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
