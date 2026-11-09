package core

import "testing"

func TestInvitationTitle(t *testing.T) {
	for _, title := range []string{"lucasyelizabeth", "LUCASYELIZABETH", "LucasyELIZABETH"} {
		if InvitationTitle("wedding", title, &Celebration{PartnerOne: "lucas", PartnerTwo: "ELIZABETH"}) != "Lucas y Elizabeth" {
			t.Fatalf("did not repair %q using registered names", title)
		}
	}
	for input, want := range map[string]string{
		"LucasyELIZABETH":          "Lucas y Elizabeth",
		" lucas  y ELIZABETH ":     "Lucas y Elizabeth",
		"MARÍA y josé":             "María y José",
		"Ana de la CRUZ y YULIANA": "Ana de la Cruz y Yuliana",
	} {
		if got := InvitationTitle("wedding", input); got != want || InvitationTitle("wedding", got) != got {
			t.Fatalf("%q: got %q, want %q", input, got, want)
		}
	}
	if InvitationTitle("general", " Fiesta DJ ") != "Fiesta DJ" {
		t.Fatal("changed an authored general event title")
	}
	if InvitationTitle("wedding", "Nuestra historia") != "Nuestra historia" {
		t.Fatal("changed a wedding title that is not a pair of names")
	}
}
