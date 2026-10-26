package httpapi

import "testing"

func TestGoogleAudienceIsBoundToProduct(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "legacy")
	t.Setenv("GOOGLE_GENERAL_CLIENT_ID", "events")
	t.Setenv("GOOGLE_WEDDING_CLIENT_ID", "weddings")
	if googleClientID("general") != "events" || googleClientID("wedding") != "weddings" || googleClientID("other") != "" {
		t.Fatal("Google audiences must be isolated per product")
	}
	t.Setenv("GOOGLE_WEDDING_CLIENT_ID", "")
	if googleClientID("wedding") != "" {
		t.Fatal("missing product client fell back to legacy audience")
	}
}
