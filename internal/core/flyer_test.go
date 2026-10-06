package core

import (
	"math"
	"testing"
)

func TestCanvasRejectsUnsafeGeometryAndAssets(t *testing.T) {
	base := FlyerCanvas{Width: 720, Height: 960, Background: "#fff9ed", Elements: []FlyerElement{{ID: "text", Type: "text", X: 10, Y: 10, Width: 400, Height: 120, Rotation: 30, Text: "Hello", Color: "#7451ac", Font: "serif", FontSize: 40, Align: "center"}}}
	if !ValidCanvas(&base, nil) {
		t.Fatal("valid canvas rejected")
	}
	cases := []func(*FlyerCanvas){
		func(c *FlyerCanvas) { c.Elements[0].X = math.NaN() },
		func(c *FlyerCanvas) { c.Elements[0].Width = 900 },
		func(c *FlyerCanvas) { c.Elements[0].Font = "url(unsafe)" },
		func(c *FlyerCanvas) { c.Elements[0].Color = "red;display:none" },
		func(c *FlyerCanvas) {
			c.Elements[0].Type = "image"
			c.Elements[0].Text = ""
			c.Elements[0].PhotoKey = "stolen.jpg"
		},
		func(c *FlyerCanvas) {
			c.Elements[0].Type = "icon"
			c.Elements[0].Text = ""
			c.Elements[0].Icon = "<script>"
		},
		func(c *FlyerCanvas) { c.Elements = append(c.Elements, c.Elements[0]) },
	}
	for n, change := range cases {
		c := base
		c.Elements = append([]FlyerElement{}, base.Elements...)
		change(&c)
		if ValidCanvas(&c, []string{"ours.jpg"}) {
			t.Fatalf("invalid case %d accepted", n)
		}
	}
}
func TestFlyerRequiresCanvasAndLegacyDesignRemainsValid(t *testing.T) {
	design := DesignInput{Title: "Our event", Template: "classic", AccentColor: "#7451ac", Sections: []Section{{ID: "one", Icon: "heart"}}}
	if !ValidDesign(design, nil) {
		t.Fatal("legacy design rejected")
	}
	design.DesignMode = "flyer"
	if ValidDesign(design, nil) {
		t.Fatal("missing flyer canvas accepted")
	}
	design.DesignMode = "unsafe"
	if ValidDesign(design, nil) {
		t.Fatal("unknown mode accepted")
	}
}
