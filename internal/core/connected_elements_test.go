package core

import "testing"

func TestConnectedElementValidation(t *testing.T) {
	lat, lon := -12.1, -77.2
	base := FlyerElement{ID: "element", Type: "text", X: 20, Y: 20, Width: 500, Height: 300, Font: "serif", FontSize: 32, Align: "center", Color: "#765432"}
	for _, binding := range []string{"couple_names", "event_title", "event_date", "map"} {
		item := base
		item.Binding = binding
		item.EventBound = true
		if binding == "map" {
			item.MapLatitude, item.MapLongitude, item.MapStyle, item.MapZoom = &lat, &lon, "warm", 14
			item.ActionHint, item.HintIcon, item.HintText = true, "external", "Cómo llegar"
		}
		canvas := &FlyerCanvas{Width: 720, Height: 960, Background: "#ffffff", Elements: []FlyerElement{item}}
		if !ValidCanvas(canvas, nil) {
			t.Fatalf("valid %s rejected", binding)
		}
		if binding == "map" {
			canvas.Elements[0].MapRadius = 500
			if ValidCanvas(canvas, nil) {
				t.Fatal("unbounded map radius accepted")
			}
			canvas.Elements[0] = item
			bad := 91.0
			canvas.Elements[0].MapLatitude = &bad
			if ValidCanvas(canvas, nil) {
				t.Fatal("invalid map coordinate")
			}
		}
	}
}

func TestGeneralEventsCannotBorrowWeddingIdentity(t *testing.T) {
	for _, title := range []string{"Cumpleaños de Javier", "Bautizo de Sofía", "Graduación de Laura"} {
		if err := ValidateProductContent("general", title, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, title := range []string{"Nuestra boda", "Nos casamos", "WEDDING de Ana y Luis"} {
		if ValidateProductContent("general", title, nil, nil) == nil {
			t.Fatal("wedding title accepted")
		}
	}
	if ValidateProductContent("general", "Evento", &Celebration{PartnerOne: "Ana"}, nil) == nil {
		t.Fatal("wedding fields accepted")
	}
	sections := []Section{{Canvas: &FlyerCanvas{Elements: []FlyerElement{{Type: "text", Binding: "couple_names"}}}}}
	if ValidateProductContent("general", "Evento", nil, sections) == nil {
		t.Fatal("wedding element accepted")
	}
	if ValidateProductContent("wedding", "Nuestra boda", &Celebration{PartnerOne: "Ana"}, sections) != nil {
		t.Fatal("wedding product rejected")
	}
	for _, kind := range []string{"general", "wedding"} {
		if _, err := InvitationHost(kind, "save"); err == nil {
			t.Fatal("reserved product domain accepted")
		}
	}
}
