package core

import (
	"strings"
	"testing"
)

func TestInvitationMessage(t *testing.T) {
	link := "https://fiesta.thedate.now/rsvp/personal"
	prompt := PollInstruction
	for _, tc := range []struct{ in, want string }{
		{"", "Abre tu invitación:\n" + link + "\n\n" + prompt},
		{"Antes{{link}}Después", "Abre tu invitación:\n" + link + "\n\nAntesDespués\n\n" + prompt},
		{" Ven a celebrar ", "Abre tu invitación:\n" + link + "\n\nVen a celebrar\n\n" + prompt},
		{"Confirma en la encuesta aquí: {{enlace}}. Te esperamos.", "Abre tu invitación:\n" + link + "\n\nConfirma en la encuesta aquí: . Te esperamos."},
		{strings.Repeat("Mucho texto. ", 300) + "{{link}}", "Abre tu invitación:\n" + link + "\n\n" + strings.TrimSpace(strings.Repeat("Mucho texto. ", 300)) + "\n\n" + prompt},
	} {
		if got := InvitationMessage(tc.in, link); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
}
func TestCelebrationDressDefaults(t *testing.T) {
	c := &Celebration{DressExcludedColors: []DressExcludedColor{{Name: "Blanco", Color: "#FFFFFF"}}}
	if !ValidCelebration(c) {
		t.Fatal("valid defaults rejected")
	}
	c.DressExcludedColors[0].Color = "invalid"
	if ValidCelebration(c) {
		t.Fatal("invalid defaults accepted")
	}
}

func TestCelebrationRestrictionDefaults(t *testing.T) {
	c := &Celebration{RestrictionText: "Sin mascotas.", RestrictionIcon: "lucide:paw-print"}
	if !ValidCelebration(c) {
		t.Fatal("valid restrictions rejected")
	}
	c.RestrictionIcon = "javascript:invalid"
	if ValidCelebration(c) {
		t.Fatal("invalid icon accepted")
	}
}
