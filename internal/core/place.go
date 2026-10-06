package core

import (
	"net/url"
	"strings"
	"time"
	_ "time/tzdata"
)

func GoogleMapsURL(raw string) bool {
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

func ValidHTTPSURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Port() == "" && !strings.ContainsAny(raw, "\r\n")
}

func ValidEventPlace(in EventInput) bool {
	if len(strings.TrimSpace(in.Organizer)) < 2 || len(in.Organizer) > 120 || len(in.Location) > 500 || len(in.MapURL) > 2048 || len(in.VirtualURL) > 2048 {
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
		return ValidHTTPSURL(in.VirtualURL)
	}
	return strings.TrimSpace(in.Location) != "" && GoogleMapsURL(in.MapURL)
}

func EventLocation(in EventInput) string {
	if in.IsVirtual {
		return "En línea"
	}
	return strings.TrimSpace(in.Location)
}

func PhysicalMapURL(in EventInput) string {
	if in.IsVirtual {
		return ""
	}
	return strings.TrimSpace(in.MapURL)
}

func VirtualEventURL(in EventInput) string {
	if !in.IsVirtual {
		return ""
	}
	return strings.TrimSpace(in.VirtualURL)
}
