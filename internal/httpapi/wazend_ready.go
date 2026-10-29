package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func wazendSessionReady(ctx context.Context, config wazendSettings) bool {
	if !config.ready() {
		return false
	}
	base, err := url.Parse(config.Base)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(config.Base, "/")+"/api/sessions/"+url.PathEscape(config.Session), nil)
	if err != nil {
		return false
	}
	req.Header.Set("X-Api-Key", config.Key)
	resp, err := wazendClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var session struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	return resp.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&session) == nil && session.Name == config.Session && session.Status == "WORKING"
}
