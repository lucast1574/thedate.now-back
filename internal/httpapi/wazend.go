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
	link := "https://" + host + "/rsvp/" + g.InviteToken
	return map[string]any{
		"chatId": strings.TrimPrefix(g.Phone, "+") + "@c.us",
		"event": map[string]any{
			"name":               invitationBrand(e.Kind) + " · " + e.Title,
			"description":        core.InvitationMessage(e.InvitationMessage, link) + core.WhatsAppCelebration(e.Celebration),
			"startTime":          e.StartAt.Unix(),
			"endTime":            e.StartAt.Add(4 * time.Hour).Unix(),
			"location":           map[string]string{"name": e.Location},
			"extraGuestsAllowed": false,
		},
	}
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
