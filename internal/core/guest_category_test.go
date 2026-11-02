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

func TestGuestCategoriesFollowTheProduct(t *testing.T) {
	for _, value := range []string{"Familia", "Amigos", "Compañeros de trabajo", "Compañeros de estudios", "Vecinos", "Equipo o club", "Invitados especiales", "Otros"} {
		if got, ok := GuestCategory(value, "general"); !ok || got != value {
			t.Fatal(value, got, ok)
		}
	}
	if _, ok := GuestCategory("Familia del novio", "general"); ok {
		t.Fatal("wedding group accepted for celebration")
	}
	if _, ok := GuestCategory("Vecinos", "wedding"); ok {
		t.Fatal("celebration group accepted for wedding")
	}
	if got, ok := GuestCategory(" COMPANEROS  DE ESTUDIOS ", "general"); !ok || got != "Compañeros de estudios" {
		t.Fatal(got, ok)
	}
}
