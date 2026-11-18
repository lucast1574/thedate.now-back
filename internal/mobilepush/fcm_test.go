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

func TestWebMessageRoutesToTheMatchingStudio(t *testing.T) {
	for project, host := range map[string]string{
		"the-date-5c49a":      "https://crea.thedate.now",
		"save-the-date-b7670": "https://studio.save.thedate.now",
	} {
		t.Run(project, func(t *testing.T) {
			c := Client{Project: project, HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				var p struct {
					Message struct {
						FID     string            `json:"fid"`
						Data    map[string]string `json:"data"`
						WebPush struct {
							Options struct {
								Link string `json:"link"`
							} `json:"fcm_options"`
						} `json:"webpush"`
					} `json:"message"`
				}
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Fatal(err)
				}
				if p.Message.FID != "installation" || len(p.Message.Data) != 2 || p.Message.Data["screen"] != "messages" {
					t.Fatalf("unexpected push target or routing: %+v", p.Message)
				}
				if link := p.Message.WebPush.Options.Link; link != host+"/manager/event?tab=messages" {
					t.Fatalf("unexpected studio link: %s", link)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"name":"message"}`))}, nil
			})}}
			if invalid, err := c.Send(context.Background(), "installation", core.MobileChange{ID: "change", EventID: "event", Screen: "messages"}); invalid || err != nil {
				t.Fatalf("send failed: invalid %v, err %v", invalid, err)
			}
		})
	}
}
