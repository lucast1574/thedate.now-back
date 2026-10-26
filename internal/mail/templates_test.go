package mail

import (
	"strings"
	"testing"
)

func TestReactEmailTemplatesAreBrandedEscapedAndHavePlainText(t *testing.T) {
	for _, product := range []string{"general", "wedding"} {
		for _, kind := range []string{"welcome", "ticket_received", "ticket_resolved", "invitation", "payment_received"} {
			content, err := Render(product, kind, TemplateData{Name: "<script>bad</script>", Title: "<b>Private title</b>", Inviter: "<em>Team</em>", ActionURL: "https://crea.thedate.now/support"})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(content.HTML, "<script>") || strings.Contains(content.HTML, "<b>Private title</b>") || strings.Contains(content.HTML, "{{") {
				t.Fatal("template contains unescaped values or unresolved placeholders")
			}
			logo := "event-icon.png"
			if product == "wedding" {
				logo = "wedding-icon.png"
			}
			if !strings.Contains(content.HTML, logo) || !strings.Contains(content.HTML, "@media") || !strings.Contains(content.Text, "https://crea.thedate.now/support") {
				t.Fatal("missing brand, responsive styles or plain-text action")
			}
			if len(content.HTML) > 80000 {
				t.Fatal("email exceeds client clipping budget")
			}
		}
	}
	if _, err := Render("general", "welcome", TemplateData{ActionURL: "javascript:alert(1)"}); err == nil {
		t.Fatal("unsafe action accepted")
	}
}
