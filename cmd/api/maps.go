package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var mapsCoordinates = []*regexp.Regexp{
	regexp.MustCompile(`!3d(-?\d+(?:\.\d+)?)!4d(-?\d+(?:\.\d+)?)`),
	regexp.MustCompile(`/@(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)`),
	regexp.MustCompile(`[?&](?:q|query|loc)=loc:(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)`),
	regexp.MustCompile(`[?&](?:q|query|ll|center|destination)=(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)`),
}

var geocodeMu sync.Mutex
var lastGeocode time.Time

func googleMapsURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "maps.app.goo.gl", "share.google":
		return u.Path != "" && u.Path != "/"
	case "goo.gl":
		return strings.HasPrefix(u.Path, "/maps/")
	case "g.co":
		return strings.HasPrefix(u.Path, "/kgs/") || strings.HasPrefix(u.Path, "/maps/")
	case "maps.google.com":
		return true
	case "google.com", "www.google.com":
		return strings.HasPrefix(u.Path, "/maps")
	default:
		return false
	}
}

func googleRedirectURL(raw string) bool {
	if googleMapsURL(raw) {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && u.Hostname() == "consent.google.com"
}

func validHTTPSURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Port() == "" && !strings.ContainsAny(raw, "\r\n")
}

func validEventPlace(in eventInput) bool {
	if len(strings.TrimSpace(in.Organizer)) < 2 {
		return false
	}
	if len(in.TimeZone) > 64 {
		return false
	}
	if in.TimeZone != "" {
		if _, err := time.LoadLocation(in.TimeZone); err != nil {
			return false
		}
	}
	if in.Kind == "wedding" && in.IsVirtual {
		return false
	}
	if in.IsVirtual {
		return validHTTPSURL(in.VirtualURL)
	}
	return strings.TrimSpace(in.Location) != "" && googleMapsURL(in.MapURL)
}

func eventLocation(in eventInput) string {
	if in.IsVirtual {
		return "En línea"
	}
	return strings.TrimSpace(in.Location)
}

func physicalMapURL(in eventInput) string {
	if in.IsVirtual {
		return ""
	}
	return strings.TrimSpace(in.MapURL)
}

func virtualEventURL(in eventInput) string {
	if !in.IsVirtual {
		return ""
	}
	return strings.TrimSpace(in.VirtualURL)
}

func (s *server) completeLegacyEvent(ctx context.Context, event *core.Event) {
	if strings.TrimSpace(event.Organizer) == "" {
		var owner struct {
			Name string `bson:"name"`
		}
		if s.db.Collection("users").FindOne(ctx, bson.M{"_id": event.OwnerID}).Decode(&owner) == nil {
			event.Organizer = owner.Name
		}
		if strings.TrimSpace(event.Organizer) == "" {
			event.Organizer = event.Title
		}
	}
	if !event.IsVirtual && event.MapURL == "" && strings.TrimSpace(event.Location) != "" {
		event.MapURL = "https://www.google.com/maps/search/?api=1&query=" + url.QueryEscape(event.Location)
	}
}

func extractMapsCoordinates(raw string) (float64, float64, bool) {
	decoded, err := url.QueryUnescape(raw)
	if err == nil {
		raw = decoded
	}
	for _, pattern := range mapsCoordinates {
		match := pattern.FindStringSubmatch(raw)
		if len(match) != 3 {
			continue
		}
		lat, errLat := strconv.ParseFloat(match[1], 64)
		lng, errLng := strconv.ParseFloat(match[2], 64)
		if errLat == nil && errLng == nil && lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180 {
			return lat, lng, true
		}
	}
	return 0, 0, false
}

func resolveMapsCoordinates(ctx context.Context, raw string) (float64, float64, bool) {
	if lat, lng, ok := extractMapsCoordinates(raw); ok {
		return lat, lng, true
	}
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 6 || !googleRedirectURL(req.URL.String()) {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return 0, 0, false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TheDate/1.0; +https://thedate.now)")
	response, err := client.Do(req)
	if err != nil {
		return 0, 0, false
	}
	defer response.Body.Close()
	if lat, lng, ok := extractMapsCoordinates(response.Request.URL.String()); ok {
		return lat, lng, true
	}
	if response.Request.URL.Hostname() == "consent.google.com" {
		continued := response.Request.URL.Query().Get("continue")
		if googleMapsURL(continued) {
			if lat, lng, ok := extractMapsCoordinates(continued); ok {
				return lat, lng, true
			}
		}
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		if location := response.Header.Get("Location"); location != "" {
			if next, err := response.Request.URL.Parse(location); err == nil && googleRedirectURL(next.String()) {
				return extractMapsCoordinates(next.String())
			}
		}
	}
	if response.StatusCode != http.StatusOK {
		return 0, 0, false
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	if err != nil {
		return 0, 0, false
	}
	return extractMapsCoordinates(html.UnescapeString(string(body)))
}

func (s *server) reverseGeocode(ctx context.Context, lat, lng float64) string {
	key := fmt.Sprintf("%.6f,%.6f", lat, lng)
	var cached struct {
		Address string `bson:"address"`
	}
	if s.db.Collection("geocodes").FindOne(ctx, bson.M{"_id": key}).Decode(&cached) == nil && cached.Address != "" {
		return cached.Address
	}
	geocodeMu.Lock()
	defer geocodeMu.Unlock()
	if wait := time.Until(lastGeocode.Add(time.Second)); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return key
		}
	}
	lastGeocode = time.Now()
	base := env("NOMINATIM_BASE_URL", "https://nominatim.openstreetmap.org")
	endpoint, err := url.Parse(strings.TrimRight(base, "/") + "/reverse")
	if err != nil {
		return key
	}
	query := endpoint.Query()
	query.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
	query.Set("lon", strconv.FormatFloat(lng, 'f', 6, 64))
	query.Set("format", "json")
	query.Set("accept-language", "es")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return key
	}
	req.Header.Set("User-Agent", "TheDate/1.0 (https://thedate.now)")
	response, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return key
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return key
	}
	var result struct {
		DisplayName string `json:"display_name"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 32<<10)).Decode(&result) != nil || result.DisplayName == "" {
		return key
	}
	address := strings.TrimSpace(result.DisplayName)
	_, _ = s.db.Collection("geocodes").InsertOne(ctx, bson.M{"_id": key, "address": address})
	return address
}

func (s *server) resolveMapsURL(w http.ResponseWriter, r *http.Request) {
	if _, err := s.user(r); err != nil {
		bad(w, http.StatusUnauthorized, "Sign in required")
		return
	}
	var in struct {
		URL string `json:"url"`
	}
	if decode(r, &in) != nil || len(in.URL) > 2048 || !googleMapsURL(in.URL) {
		bad(w, http.StatusBadRequest, "Pega un enlace válido de Google Maps")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	lat, lng, ok := resolveMapsCoordinates(ctx, strings.TrimSpace(in.URL))
	if !ok {
		bad(w, http.StatusUnprocessableEntity, "No pude obtener la dirección automáticamente. Escríbela manualmente y conserva el enlace de Google Maps.")
		return
	}
	address := s.reverseGeocode(ctx, lat, lng)
	reply(w, http.StatusOK, map[string]any{"address": address, "latitude": lat, "longitude": lng, "attribution": "© OpenStreetMap contributors"})
}
