package core

import (
	"go.mongodb.org/mongo-driver/v2/bson"
	"testing"
)

func TestRestrictionPresentationAndIcons(t *testing.T) {
	for _, style := range []string{"", "icon", "text", "icon-text"} {
		el := FlyerElement{Type: "text", Binding: "restrictions", RestrictionStyle: style, RestrictionIcon: "lucide:paw-print", Text: "Sin mascotas"}
		if !validElementBinding(el) || !ValidRestrictions(el) {
			t.Fatal("valid notice rejected", style)
		}
	}
	for _, el := range []FlyerElement{{Binding: "restrictions", RestrictionStyle: "bad"}, {Binding: "restrictions", RestrictionIcon: "unknown"}, {Binding: "restrictions", RestrictionIcon: "none"}, {Binding: "dress_code", RestrictionIcon: "lucide:paw-print"}} {
		if ValidRestrictions(el) {
			t.Fatal("invalid restriction accepted", el)
		}
	}
	if validElementBinding(FlyerElement{Type: "image", Binding: "restrictions"}) {
		t.Fatal("image binding accepted")
	}
}

func TestDressExclusionsAndRestrictionStorage(t *testing.T) {
	dress := FlyerElement{Binding: "dress_code", DressExcludedColors: []DressExcludedColor{{Name: "Blanco", Color: "#FFFFFF"}, {Name: "Marfil", Color: "#F5EFE2"}}}
	if !ValidDressCode(dress, nil) {
		t.Fatal("valid colors rejected")
	}
	for _, colors := range [][]DressExcludedColor{{{Name: " ", Color: "#FFFFFF"}}, {{Name: "Blanco", Color: "red"}}, make([]DressExcludedColor, 9)} {
		dress.DressExcludedColors = colors
		if ValidDressCode(dress, nil) {
			t.Fatal("invalid colors accepted", colors)
		}
	}
	dress.DressExcludedColors = []DressExcludedColor{{Name: "Marfil", Color: "#F5EFE2"}}
	notice := FlyerElement{Binding: "restrictions", RestrictionIcon: "lucide:camera-off", RestrictionStyle: "icon-text", Text: "Sin fotografías"}
	encoded, err := bson.Marshal(FlyerCanvas{Elements: []FlyerElement{dress, notice}})
	if err != nil {
		t.Fatal(err)
	}
	var stored FlyerCanvas
	if err = bson.Unmarshal(encoded, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Elements[0].DressExcludedColors[0].Name != "Marfil" || stored.Elements[1].RestrictionIcon != "lucide:camera-off" || stored.Elements[1].Text != "Sin fotografías" {
		t.Fatal("fields lost in storage", stored)
	}
}
