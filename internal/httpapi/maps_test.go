package httpapi

import "testing"

func TestGoogleMapsLinks(t *testing.T) {
	for _, raw := range []string{
		"https://www.google.com/maps/place/Bogota/@4.710989,-74.072090,17z",
		"https://maps.app.goo.gl/abc123",
		"https://share.google/abc123",
		"https://www.google.com/maps/search/?api=1&query=Bogota",
	} {
		if !googleMapsURL(raw) {
			t.Errorf("expected Google Maps URL to be accepted: %s", raw)
		}
	}
	for _, raw := range []string{
		"https://www.google.com.evil.example/maps/place/Foo",
		"https://127.0.0.1/maps/place/Foo",
		"https://goo.gl.evil.example/maps/Foo",
		"javascript:alert(1)",
		"http://www.google.com/maps/place/Foo",
	} {
		if googleMapsURL(raw) {
			t.Errorf("unsafe URL accepted: %s", raw)
		}
	}
}

func TestExtractMapsCoordinates(t *testing.T) {
	for _, raw := range []string{
		"https://www.google.com/maps/place/Foo/@4.711,-74.072,17z/data=!3d4.71!4d-74.07",
		"https://www.google.com/maps?q=4.71%2C-74.07",
		"https://maps.google.com/?ll=4.71,-74.07",
	} {
		lat, lng, ok := extractMapsCoordinates(raw)
		if !ok || lat != 4.71 || lng != -74.07 {
			t.Errorf("coordinates = %v, %v, %v for %s", lat, lng, ok, raw)
		}
	}
	if _, _, ok := extractMapsCoordinates("https://www.google.com/maps?q=95,210"); ok {
		t.Fatal("out of bounds coordinates accepted")
	}
}

func TestWeddingCannotBeVirtual(t *testing.T) {
	input := eventInput{Kind: "wedding", Organizer: "Laura", IsVirtual: true, VirtualURL: "https://meet.google.com/abc"}
	if validEventPlace(input) {
		t.Fatal("virtual wedding accepted")
	}
	input.Kind = "general"
	if !validEventPlace(input) {
		t.Fatal("virtual general event rejected")
	}
}
