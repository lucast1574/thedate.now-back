package core

import "testing"

func TestGiftLinkBinding(t *testing.T) {
	for _, link := range []string{"/regalos", "https://store.example/lista?pareja=ana#regalos"} {
		el := FlyerElement{Type: "text", Binding: "gift_link", Link: link}
		if !validElementBinding(el) {
			t.Errorf("valid gift link rejected: %s", link)
		}
		el.Binding = "dress_code"
		if validElementBinding(el) {
			t.Error("gift URL accepted on dress code")
		}
	}
	for _, link := range []string{"", "/regalos/token", "//example.com", "javascript:alert(1)", "data:text/html,test", "http://example.com", "https://user:password@example.com", "https://example.com/a b", "https://example.com\\evil"} {
		if ValidGiftLink(link) {
			t.Errorf("invalid gift link accepted: %q", link)
		}
	}
	el := FlyerElement{Type: "image", Binding: "gift_link", Link: "/regalos"}
	if validElementBinding(el) {
		t.Error("binding accepted on image")
	}
	el.Type = "text"
	el.Address = "injected"
	if validElementBinding(el) {
		t.Error("location metadata accepted on gift")
	}
}
