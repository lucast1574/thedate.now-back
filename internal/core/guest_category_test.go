package core

import "testing"

func TestGuestCategoryUsesOnlyKnownGroups(t *testing.T) {
	for input, expected := range map[string]string{"": "", " familia  DEL novio ": "Familia del novio", "COMPANEROS DE TRABAJO": "Compañeros de trabajo", "Amigos": "Amigos"} {
		got, ok := GuestCategory(input)
		if !ok || got != expected {
			t.Fatalf("category %q: %q %v", input, got, ok)
		}
	}
	if _, ok := GuestCategory("Familia Rojas"); ok {
		t.Fatal("created arbitrary category")
	}
}
