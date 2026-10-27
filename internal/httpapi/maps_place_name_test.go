package httpapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMapsPlaceName(t *testing.T) {
	for _, tc := range []struct{ url, name string }{
		{"https://www.google.com/maps/place/Villa+Luis+Adolfo+II/@-8.081479,-79.0983623,17z/data=!3d-8.0782241!4d-79.0976535", "Villa Luis Adolfo II"},
		{"https://www.google.com/maps/place/Sal%C3%B3n+Los+Olivos/@-8,-79,17z", "Salón Los Olivos"},
		{"https://www.google.com/maps/place/-8.078,-79.097/@-8,-79,17z", ""},
		{"https://evil.example/maps/place/Fake/@-8,-79,17z", ""},
		{"https://www.google.com/maps/place/%3Cscript%3E/@-8,-79,17z", ""},
	} {
		if got := extractMapsPlaceName(tc.url); got != tc.name {
			t.Errorf("got %q want %q", got, tc.name)
		}
	}
}
func TestVenueAndAddressStayTogetherWithoutDuplicatingName(t *testing.T) {
	if got := mapsDisplayAddress("Villa Luis Adolfo II", "Huanchaco, Trujillo, Perú"); got != "Villa Luis Adolfo II · Huanchaco, Trujillo, Perú" {
		t.Fatal(got)
	}
	if got := mapsDisplayAddress("Villa Luis Adolfo II", "Villa Luis Adolfo II, Huanchaco"); got != "Villa Luis Adolfo II, Huanchaco" {
		t.Fatal(got)
	}
	if got := mapsDisplayAddress("", "Huanchaco"); got != "Huanchaco" {
		t.Fatal(got)
	}
}

type mapsTestTransport func(*http.Request) (*http.Response, error)

func (transport mapsTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return transport(r)
}
func TestSharedMapsLinkKeepsRedirectedVenueName(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = mapsTestTransport(func(r *http.Request) (*http.Response, error) {
		response := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: r}
		if r.URL.Hostname() == "maps.app.goo.gl" {
			response.StatusCode = 302
			response.Header.Set("Location", "https://www.google.com/maps/place/Villa+Luis+Adolfo+II/@-8.081479,-79.0983623,17z/data=!3d-8.0782241!4d-79.0976535")
		}
		return response, nil
	})
	lat, lng, name, ok := resolveMapsLocation(context.Background(), "https://maps.app.goo.gl/example")
	if !ok || name != "Villa Luis Adolfo II" || lat != -8.0782241 || lng != -79.0976535 {
		t.Fatalf("resolved %f %f %q %v", lat, lng, name, ok)
	}
}
