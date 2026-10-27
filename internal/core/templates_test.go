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
	if len(seen) != 54 {
		t.Fatal("expected fifty-four distinct templates including legacy section designs")
	}
	if TemplateID("wedding", "classic", "", "") != "wedding-classic" {
		t.Fatal("legacy default lost")
	}
	for _, kind := range []string{"wedding", "general"} {
		count := 0
		for _, item := range TemplateCatalog() {
			if item.Kind == kind && item.Mode == "flyer" {
				count++
				if item.Layout == "" || item.Category == "" {
					t.Fatal("canvas metadata missing")
				}
			}
		}
		if count != 24 {
			t.Fatalf("%s has %d canvas designs", kind, count)
		}
	}
}

func TestTemplateDecorationsAreValidCanvasElements(t *testing.T) {
	for _, icon := range []string{"frame", "arch", "leaf", "sunburst"} {
		canvas := &FlyerCanvas{Width: 720, Height: 960, Background: "#fff9ed", Elements: []FlyerElement{{ID: "decoration", Type: "icon", Icon: icon, X: 40, Y: 40, Width: 640, Height: 880, Color: "#826148", Font: "serif", FontSize: 160, Align: "center"}}}
		if !ValidCanvas(canvas, nil) {
			t.Fatalf("decoration %s rejected", icon)
		}
	}
}
