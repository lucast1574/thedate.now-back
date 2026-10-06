package core

import "testing"

func TestTemplateProductAndModeAreAuthoritative(t *testing.T) {
	seen := map[string]bool{}
	for _, item := range TemplateCatalog() {
		if seen[item.ID] || item.ID == "" {
			t.Fatal("duplicate catalog ID")
		}
		seen[item.ID] = true
		if TemplateID(item.Kind, item.Style, item.Mode, item.ID) != item.ID {
			t.Fatal("valid catalog entry rejected")
		}
		other := "general"
		if item.Kind == other {
			other = "wedding"
		}
		if TemplateID(other, item.Style, item.Mode, item.ID) != "" {
			t.Fatal("cross-product template accepted")
		}
		wrongMode := "sections"
		if item.Mode == wrongMode {
			wrongMode = "flyer"
		}
		if TemplateID(item.Kind, item.Style, wrongMode, item.ID) != "" {
			t.Fatal("wrong mode accepted")
		}
	}
	if len(seen) != 12 {
		t.Fatal("expected twelve distinct templates")
	}
	if TemplateID("wedding", "classic", "", "") != "wedding-classic" {
		t.Fatal("legacy default lost")
	}
}
