package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"strings"
	"testing"
)

func TestPublicationStatusNeverRevealsAddressBeforeVerified(t *testing.T) {
	s := &server{}
	t.Setenv("DOKPLOY_URL", "")
	for _, phase := range []string{"pending", "configured", "deploying", "ready", "error"} {
		d := core.Deployment{EventID: "event", Host: "sofiaymate.save.thedate.now", ProjectID: "private-project", ApplicationID: "private-app", Image: "private-image", Phase: phase, Error: "internal provider secret"}
		view := s.publicationView(context.Background(), core.Event{ID: "event", Kind: "wedding", Slug: "sofiaymate"}, d)
		raw, _ := json.Marshal(view)
		for _, secret := range []string{"host", "sofiaymate", "private-", "internal provider"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("premature status disclosed %s", secret)
			}
		}
		if phase == "ready" && view.Phase != "verifying" {
			t.Fatal("ready container treated as public page")
		}
	}
}
