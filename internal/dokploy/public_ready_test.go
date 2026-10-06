package dokploy

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type publicTransport func(*http.Request) (*http.Response, error)

func (f publicTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPublicReadyRequiresAccessibleCorrectInvitationInBothProducts(t *testing.T) {
	now := time.Now()
	for _, kind := range []string{"wedding", "general"} {
		host, _ := core.InvitationHost(kind, "sofiaymate")
		e := core.Event{ID: "event", Kind: kind, Slug: "sofiaymate", PublishedAt: &now}
		d := core.Deployment{EventID: e.ID, Host: host}
		status := 200
		contentType := "text/html; charset=utf-8"
		body := `<main data-invitation-event="event">Invitation</main>`
		calls := 0
		client := &Client{HTTP: &http.Client{Transport: publicTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.String() != "https://"+host+"/" {
				t.Error("wrong public target")
			}
			return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})}}
		if !client.PublicPageReady(context.Background(), e, d) {
			t.Fatal("correct page not ready")
		}
		for _, code := range []int{202, 404, 500, 503} {
			status = code
			if client.PublicPageReady(context.Background(), e, d) {
				t.Fatal("unavailable page revealed link")
			}
		}
		status = 200
		body = `<main data-invitation-event="another">Wrong event</main>`
		if client.PublicPageReady(context.Background(), e, d) {
			t.Fatal("another event accepted")
		}
		body = `<html>Service is running</html>`
		if client.PublicPageReady(context.Background(), e, d) {
			t.Fatal("generic health page accepted")
		}
		body = `<main data-invitation-event="event">OK</main>`
		contentType = "application/json"
		if client.PublicPageReady(context.Background(), e, d) {
			t.Fatal("non-page accepted")
		}
		contentType = "text/html"
		e.PublishedAt = nil
		if client.PublicPageReady(context.Background(), e, d) {
			t.Fatal("unpublished event accepted")
		}
		e.PublishedAt = &now
		d.Host = "localhost"
		before := calls
		if client.PublicPageReady(context.Background(), e, d) || calls != before {
			t.Fatal("arbitrary host requested")
		}
	}
}
