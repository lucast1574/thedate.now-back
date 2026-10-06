package core

import "testing"

func TestGuestBindingOnlyAcceptsSafeTextElements(t *testing.T) {
	name := FlyerElement{ID: "name", Type: "text", Binding: "guest_name", Text: "Hola, {{nombre_invitado}}", Width: 720, Height: 160, Y: 40, Color: "#995d72", Font: "script", FontSize: 48, Align: "center"}
	in := DesignInput{Title: "Our day", Template: "classic", AccentColor: "#995d72", Sections: []Section{{ID: "s", Icon: "none", GuestText: &name}}}
	if !ValidDesign(in, nil) {
		t.Fatal("personal text rejected")
	}
	name.Binding = "phone"
	if ValidDesign(in, nil) {
		t.Fatal("unapproved private binding accepted")
	}
	name.Binding = "guest_name"
	name.Type = "image"
	if ValidDesign(in, nil) {
		t.Fatal("non-text binding accepted")
	}
	if GuestFullName(Guest{Name: " Ana ", LastName: " Rojas "}) != "Ana Rojas" {
		t.Fatal("full name formatting")
	}
}
