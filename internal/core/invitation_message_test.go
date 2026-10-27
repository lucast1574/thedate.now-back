package core

import "testing"

func TestInvitationMessage(t *testing.T) {
	link := "https://fiesta.thedate.now/rsvp/personal"
	for _, tc := range []struct{ in, want string }{
		{"", link},
		{" Ven a celebrar ", "Ven a celebrar\n\n" + link},
		{"Confirma: {{enlace}}. Te esperamos.", "Confirma: " + link + ". Te esperamos."},
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
