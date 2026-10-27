package httpapi

import (
	"context"
	"html"
	"io"
	"net/http"
	"time"
)

func resolveMapsLocation(ctx context.Context, raw string) (float64, float64, string, bool) {
	if lat, lng, ok := extractMapsCoordinates(raw); ok {
		return lat, lng, extractMapsPlaceName(raw), true
	}
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 6 || !googleRedirectURL(req.URL.String()) {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return 0, 0, "", false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TheDate/1.0; +https://thedate.now)")
	response, err := client.Do(req)
	if err != nil {
		return 0, 0, "", false
	}
	defer response.Body.Close()
	if lat, lng, ok := extractMapsCoordinates(response.Request.URL.String()); ok {
		return lat, lng, extractMapsPlaceName(response.Request.URL.String()), true
	}
	if response.Request.URL.Hostname() == "consent.google.com" {
		continued := response.Request.URL.Query().Get("continue")
		if googleMapsURL(continued) {
			if lat, lng, ok := extractMapsCoordinates(continued); ok {
				return lat, lng, extractMapsPlaceName(continued), true
			}
		}
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		if location := response.Header.Get("Location"); location != "" {
			if next, err := response.Request.URL.Parse(location); err == nil && googleRedirectURL(next.String()) {
				lat, lng, ok := extractMapsCoordinates(next.String())
				return lat, lng, extractMapsPlaceName(next.String()), ok
			}
		}
	}
	if response.StatusCode != http.StatusOK {
		return 0, 0, "", false
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	if err != nil {
		return 0, 0, "", false
	}
	lat, lng, ok := extractMapsCoordinates(html.UnescapeString(string(body)))
	return lat, lng, extractMapsPlaceName(response.Request.URL.String()), ok
}
