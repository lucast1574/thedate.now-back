package core

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

func ProfileName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 80 {
		return "", false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return name, true
}

// Only Google's HTTPS image hosts are accepted from a verified identity token.
func GooglePhoto(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return ""
	}
	if !strings.HasSuffix(u.Hostname(), ".googleusercontent.com") {
		return ""
	}
	return raw
}
