package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lucast1574/thedate.now-back/internal/core"
)

func TestPollFollowupUsesSeparateResponsePage(t *testing.T) {
	var messages []map[string]string
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var message map[string]string
		if r.URL.Path != "/api/sendText" || json.NewDecoder(r.Body).Decode(&message) != nil {
			t.Error("Invalid followup request")
		}
		messages = append(messages, message)
		w.WriteHeader(http.StatusOK)
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	for _, kind := range []string{"general", "wedding"} {
		prefix := "WAZEND_GENERAL_"
		if kind == "wedding" {
			prefix = "WAZEND_WEDDING_"
		}
		t.Setenv(prefix+"BASE_URL", provider.URL)
		t.Setenv(prefix+"API_KEY", "test")
		t.Setenv(prefix+"SESSION", "test")
		e := core.Event{Kind: kind, Slug: "celebracion"}
		g := core.Guest{Phone: "+51999111222", InviteToken: strings.Repeat("a", 48)}
		requestRSVPFollowup(e, g, "Complete your party: ")
		host, _ := core.InvitationHost(kind, e.Slug)
		if len(messages) == 0 || !strings.HasSuffix(messages[len(messages)-1]["text"], "https://"+host+"/rsvp/"+g.InviteToken+"/respuesta") {
			t.Fatal("Followup must open the response page")
		}
		if strings.Contains(eventMessage(e, g)["text"].(string), "/respuesta") {
			t.Fatal("First message must keep the invitation link")
		}
	}
}
