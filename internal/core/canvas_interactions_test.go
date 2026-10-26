package core

import "testing"

func TestCanvasLocationAndSymbolValidation(t *testing.T) {
	el := FlyerElement{ID: "one", Type: "text", Binding: "location", LocationStyle: "address", Address: "Bogotá", Link: "https://maps.app.goo.gl/example", Width: 400, Height: 120, Color: "#123456", Font: "serif", FontSize: 28, Align: "center"}
	canvas := FlyerCanvas{Width: 720, Height: 960, Background: "#ffffff", Elements: []FlyerElement{el}}
	for _, style := range []string{"address", "label", "icon", "icon-label", "icon-address", "label-address", "all"} {
		canvas.Elements[0].LocationStyle = style
		if !ValidCanvas(&canvas, nil) {
			t.Fatalf("rejects location style %s", style)
		}
	}
	for _, link := range []string{"javascript:alert(1)", "https://google.com.evil/maps", "https://google.com@evil.com/maps"} {
		canvas.Elements[0].Link = link
		if ValidCanvas(&canvas, nil) {
			t.Fatalf("accepts unsafe maps link %s", link)
		}
	}
	canvas.Elements[0] = el
	canvas.Elements[0].Binding = ""
	if ValidCanvas(&canvas, nil) {
		t.Fatal("accepts location data without binding")
	}
	canvas.Elements[0] = FlyerElement{ID: "symbol", Type: "icon", Icon: "lucide:heart-handshake", Width: 120, Height: 120, Color: "#123456", Font: "serif", FontSize: 48, Align: "center"}
	if !ValidCanvas(&canvas, nil) {
		t.Fatal("rejects bundled Lucide icon")
	}
	canvas.Elements[0].Icon = "lucide:not-an-icon"
	if ValidCanvas(&canvas, nil) {
		t.Fatal("accepts unknown icon")
	}
	if len(lucideSymbols) < 1800 {
		t.Fatal("symbol library incomplete")
	}
}
