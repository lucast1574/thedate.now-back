package dokploy

import (
	"bytes"
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"html"
	"io"
	"net/http"
	"strings"
)

// PublicPageReady checks the actual rendered page, not just the container health.
// Host must come from the event policy, never from arbitrary client input.
func (c *Client) PublicPageReady(ctx context.Context, e core.Event, d core.Deployment) bool {
	host, err := core.InvitationHost(e.Kind, e.Slug)
	if err != nil || d.Host != host || d.EventID != e.ID || e.PublishedAt == nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Scheme != "https" || resp.Request.URL.Host != host || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return false
	}
	marker := []byte(`data-invitation-event="` + html.EscapeString(e.ID) + `"`)
	return bytes.Contains(body, marker)
}
