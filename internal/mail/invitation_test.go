package mail

import (
	"strings"
	"testing"
)

func TestInvitationBrandEscapingAndScope(t *testing.T) {
	for _, kind := range []string{"wedding", "general"} {
		html := AccessInvitation(kind, "<script>bad</script>", "<b>Event</b>", "https://crea.thedate.now/join/abc")
		if strings.Contains(html, "<script>") || strings.Contains(html, "<b>Event</b>") {
			t.Fatal("unescaped email content")
		}
		if !strings.Contains(html, "https://crea.thedate.now/join/abc") || !strings.Contains(html, "7 días") {
			t.Fatal("missing acceptance link or expiry")
		}
		if kind == "wedding" && !strings.Contains(html, "Save the Date") {
			t.Fatal("wedding brand missing")
		}
		if kind == "general" && !strings.Contains(html, "The Date") {
			t.Fatal("general brand missing")
		}
	}
}
