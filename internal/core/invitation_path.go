package core

import (
	"encoding/base64"
	"encoding/hex"
	"golang.org/x/text/unicode/norm"
	"strings"
	"unicode"
)

// Keep all 192 bits of the original bearer token in a URL-safe personal address.
// The name is decorative: changing a guest's name never invalidates sent links.
func GuestInvitationPath(g Guest) string {
	token, err := hex.DecodeString(g.InviteToken)
	if err != nil || len(token) != 24 {
		return "/rsvp/" + g.InviteToken
	}
	var name strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(GuestFullName(g))) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			name.WriteRune(r)
		} else {
			name.WriteByte('-')
		}
	}
	parts := strings.FieldsFunc(name.String(), func(r rune) bool { return r == '-' })
	slug := strings.Join(parts, "-")
	if len(slug) > 60 {
		slug = strings.TrimRight(slug[:60], "-")
	}
	if slug == "" {
		slug = "invitado"
	}
	return "/para/" + slug + "-" + base64.RawURLEncoding.EncodeToString(token)
}
