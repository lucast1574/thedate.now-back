package core

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/v2/bson"
	"testing"
)

func TestDressPaletteValidationAndPersistence(t *testing.T) {
	for _, style := range []string{"", "fabric", "petal", "bar", "jewel", "wave", "ticket"} {
		original := FlyerElement{Binding: "dress_code", DressPaletteStyle: style}
		if !ValidDressCode(original, nil) {
			t.Fatal("valid palette rejected", style)
		}
		for _, format := range []string{"json", "bson"} {
			var data []byte
			var err error
			var restored FlyerElement
			if format == "json" {
				data, err = json.Marshal(original)
				if err == nil {
					err = json.Unmarshal(data, &restored)
				}
			} else {
				data, err = bson.Marshal(original)
				if err == nil {
					err = bson.Unmarshal(data, &restored)
				}
			}
			if err != nil || restored.DressPaletteStyle != style {
				t.Fatal("palette did not round trip", format, style, err)
			}
		}
	}
	for _, invalid := range []FlyerElement{{Binding: "dress_code", DressPaletteStyle: "url(javascript:x)"}, {Binding: "location", DressPaletteStyle: "petal"}} {
		if ValidDressCode(invalid, nil) {
			t.Fatal("invalid palette accepted")
		}
	}
}
