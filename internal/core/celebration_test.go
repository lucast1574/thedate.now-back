package core

import (
	"strings"
	"testing"
)

func TestCelebrationNoticeRequiresBothChoices(t *testing.T) {
	for _, c := range []*Celebration{nil, {}, {AdultsOnly: true}, {WhatsAppNotice: true}} {
		if WhatsAppCelebration(c) != "" {
			t.Fatal("sent an unselected notice")
		}
	}
	c := &Celebration{AdultsOnly: true, WhatsAppNotice: true, AdultsMessage: "  Celebración sin niños. Gracias.  "}
	if got := WhatsAppCelebration(c); got != "\n\nCelebración sin niños. Gracias." {
		t.Fatal(got)
	}
	c.AdultsMessage = ""
	if !strings.Contains(WhatsAppCelebration(c), "solo para adultos") {
		t.Fatal("missing fallback")
	}
}
func TestOpeningAndCelebrationValidation(t *testing.T) {
	valid := Opening{Enabled: true, Style: "classic", Color: "#F4EBDF", Ink: "#766044", ButtonLabel: "Abrir"}
	if !ValidOpening(&valid) || !ValidOpening(nil) || !ValidCelebration(nil) {
		t.Fatal("valid configuration rejected")
	}
	for _, style := range []string{"classic", "arch", "ticket", "curtain", "bloom", "orbit"} {
		valid.Style = style
		if !ValidOpening(&valid) {
			t.Fatal(style)
		}
	}
	valid.Style = "unknown"
	if ValidOpening(&valid) {
		t.Fatal("unknown style accepted")
	}
	valid.Style = "classic"
	valid.Color = "url(https://example.com)"
	if ValidOpening(&valid) {
		t.Fatal("unsafe color accepted")
	}
	if ValidCelebration(&Celebration{AdultsMessage: strings.Repeat("x", 501)}) {
		t.Fatal("unbounded notice")
	}
}

func TestOpeningPaletteMode(t *testing.T) {
	opening := Opening{Style: "bloom", Color: "#F4EBDF", Ink: "#766044", ButtonLabel: "Abrir"}
	for _, mode := range []string{"", "design", "custom"} {
		opening.PaletteMode = mode
		if !ValidOpening(&opening) {
			t.Fatal("supported palette rejected", mode)
		}
	}
	opening.PaletteMode = "external"
	if ValidOpening(&opening) {
		t.Fatal("unknown palette mode accepted")
	}
}
func TestOnlyBundledPhotosArePublicTemplateAssets(t *testing.T) {
	for _, key := range []string{"template-photo"} {
		if !validPhoto(key, nil) {
			t.Fatal(key)
		}
	}
	for _, key := range []string{"template-other.png", "../template-wedding.png", "https://external/photo.png", "private-owned-by-someone-else.png"} {
		if validPhoto(key, nil) {
			t.Fatal(key)
		}
	}
	if !validPhoto("my-photo.png", []string{"my-photo.png"}) {
		t.Fatal("owned photo rejected")
	}
}

func TestOpeningCustomTitleFontAndSeal(t *testing.T) {
	o := Opening{Enabled: true, Style: "classic", Color: "#FFFFFF", Ink: "#123456", ButtonLabel: "Abrir", Title: "Una carta para ti", Font: "script", SealIcon: "lucide:heart"}
	if !ValidOpening(&o) {
		t.Fatal("custom opening rejected")
	}
	o.SealIcon = "none"
	if !ValidOpening(&o) {
		t.Fatal("hidden seal rejected")
	}
	o.SealIcon = "unknown"
	if ValidOpening(&o) {
		t.Fatal("unknown seal accepted")
	}
	o.SealIcon = ""
	o.Font = "url(https://external)"
	if ValidOpening(&o) {
		t.Fatal("invalid font accepted")
	}
}
