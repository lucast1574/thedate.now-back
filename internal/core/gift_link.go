package core

import (
	"net/url"
	"strings"
	"unicode"
)

// ValidGiftLink allows the event's own list or an external HTTPS list. It never fetches the target.
func ValidGiftLink(raw string) bool {
	if raw == "/regalos" {
		return true
	}
	if raw == "" || len(raw) > 2048 || strings.ContainsRune(raw, '\\') || strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil
}
