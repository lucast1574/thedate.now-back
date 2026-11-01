package mobilepush

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSendUsesFIDAndOnlyPrivateEventRouting(t *testing.T) {
	c := Client{Project: "project", HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://fcm.googleapis.com/v1/projects/project/messages:send" {
			t.Fatal(r.URL)
		}
		var p struct {
			Message map[string]any `json:"message"`
		}
		json.NewDecoder(r.Body).Decode(&p)
		if p.Message["fid"] != "installation" || p.Message["token"] != nil {
			t.Fatal("wrong target")
		}
		data := p.Message["data"].(map[string]any)
		if len(data) != 2 || data["eventId"] != "event" || data["screen"] != "gifts" {
			t.Fatal(data)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"name":"projects/project/messages/one"}`))}, nil
	})}}
	invalid, err := c.Send(context.Background(), "installation", core.MobileChange{ID: "change", EventID: "event", Screen: "gifts"})
	if err != nil || invalid {
		t.Fatal(err)
	}
}
func TestCredentialsCannotCrossProducts(t *testing.T) {
	_, err := New(context.Background(), []byte(`{"type":"service_account","project_id":"other","token_uri":"https://oauth2.googleapis.com/token"}`), "expected")
	if err == nil {
		t.Fatal("cross product credentials accepted")
	}
}
