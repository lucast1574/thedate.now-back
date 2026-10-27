package core

import "testing"

func TestDressCodeCompositionsAndPhotoOwnership(t *testing.T) {
	for _, style := range []string{"", "text", "icon", "image", "icon-text", "image-text", "icon-image", "all"} {
		el := FlyerElement{Binding: "dress_code", DressStyle: style, DressIcon: "lucide:shirt", DressPhotoKey: "my-photo.png"}
		if !ValidDressCode(el, []string{"my-photo.png"}) {
			t.Fatal(style)
		}
		if ValidDressCode(el, nil) {
			t.Fatal("foreign photo accepted")
		}
		el.DressPhotoKey = "template-photo"
		if !ValidDressCode(el, nil) {
			t.Fatal("placeholder rejected")
		}
	}
	for _, el := range []FlyerElement{{Binding: "dress_code", DressStyle: "bad"}, {Binding: "dress_code", DressIcon: "unknown-icon"}, {Binding: "location", DressStyle: "icon"}, {Binding: "dress_code", DressPhotoKey: "https://external/photo.png"}} {
		if ValidDressCode(el, nil) {
			t.Fatal("invalid presentation accepted", el)
		}
	}
}
