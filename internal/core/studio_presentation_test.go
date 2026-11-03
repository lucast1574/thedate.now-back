package core

import (
	"encoding/json"
	"math"
	"testing"
)

func TestStudioPresentationRoundTripAndValidation(t *testing.T) {
	for _, raw := range []string{
		`{"type":"text","typeface":"display","italic":true,"lineHeight":1.4,"letterSpacing":0.08}`,
		`{"type":"image","photoPositionX":0,"photoPositionY":100,"opacity":0.5}`,
		`{"type":"icon","icon":"art:olive","opacity":0.7}`,
		`{"type":"text"}`,
	} {
		var el FlyerElement
		if err := json.Unmarshal([]byte(raw), &el); err != nil {
			t.Fatal(err)
		}
		if !validStudioPresentation(el) {
			t.Fatalf("rejected %s", raw)
		}
		encoded, _ := json.Marshal(el)
		var restored FlyerElement
		if err := json.Unmarshal(encoded, &restored); err != nil || !validStudioPresentation(restored) {
			t.Fatal("lost presentation")
		}
		if el.PhotoPositionX != nil && (restored.PhotoPositionX == nil || *restored.PhotoPositionX != 0) {
			t.Fatal("lost zero focal position")
		}
	}
	for _, value := range []float64{-1, 101, math.NaN(), math.Inf(1)} {
		if validStudioPresentation(FlyerElement{Type: "image", PhotoPositionX: &value}) {
			t.Fatal("accepted invalid focal point")
		}
	}
	if validStudioPresentation(FlyerElement{Type: "text", Typeface: "url(https://other)"}) {
		t.Fatal("accepted arbitrary font")
	}
	if !validFlyerIcon("art:olive") || validFlyerIcon("art:unknown") {
		t.Fatal("invalid ornament whitelist")
	}
}
