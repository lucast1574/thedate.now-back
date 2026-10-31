package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCraftedCanvasOptionsAreValidatedAndRoundTrip(t *testing.T) {
	el := FlyerElement{ID: "photo", Type: "image", X: 0, Y: 0, Width: 720, Height: 600, Color: "#123456", Font: "serif", FontSize: 28, Align: "center", PhotoKey: "template-photo", PhotoFit: "cover", PhotoTreatment: "soft", GroupID: "composition", Locked: true}
	canvas := FlyerCanvas{Width: 720, Height: 960, Background: "#ffffff", Font: "script", Art: "botanical", Elements: []FlyerElement{el}}
	for _, art := range []string{"botanical", "romance", "gala", "editorial", "fiesta", "coastal"} {
		canvas.Art = art
		for _, treatment := range []string{"none", "soft", "fade", "arch", "cinema"} {
			canvas.Elements[0].PhotoTreatment = treatment
			if !ValidCanvas(&canvas, nil) {
				t.Fatalf("rejected %s/%s", art, treatment)
			}
			data, err := json.Marshal(canvas)
			if err != nil {
				t.Fatal(err)
			}
			var restored FlyerCanvas
			if err = json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			if !restored.Elements[0].Locked || restored.Elements[0].GroupID != "composition" || restored.Art != art || restored.Font != "script" || restored.Elements[0].PhotoTreatment != treatment {
				t.Fatal("lost composition metadata")
			}
		}
	}
	invalid := canvas
	invalid.Art = "untrusted"
	if ValidCanvas(&invalid, nil) {
		t.Fatal("unvalidated art")
	}
	invalid = canvas
	invalid.Font = "external-font"
	if ValidCanvas(&invalid, nil) {
		t.Fatal("unvalidated font")
	}
	canvas.Elements[0].GroupID = strings.Repeat("a", 65)
	if ValidCanvas(&canvas, nil) {
		t.Fatal("unbounded group ID")
	}
	canvas.Elements[0] = el
	canvas.Elements[0].PhotoTreatment = "url(javascript:x)"
	if ValidCanvas(&canvas, nil) {
		t.Fatal("unvalidated treatment")
	}
	canvas.Elements[0] = FlyerElement{ID: "rsvp", Type: "text", Binding: "rsvp_link", Width: 400, Height: 90, Color: "#123456", Font: "serif", FontSize: 28, Align: "center", Text: "Completar mi respuesta"}
	if !ValidCanvas(&canvas, nil) {
		t.Fatal("RSVP section rejected")
	}
	canvas.Elements[0].Link = "https://example.com"
	if ValidCanvas(&canvas, nil) {
		t.Fatal("RSVP destination must be resolved from the personal invitation")
	}
}
