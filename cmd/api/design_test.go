package main

import (
	"testing"

	"github.com/lucast1574/thedate.now-back/internal/core"
)

func TestValidDesignRejectsUnownedImageAndUnsafeIcon(t *testing.T) {
	base := designInput{Title: "Our wedding", Template: "classic", AccentColor: "#aabbcc", Sections: []core.Section{{ID: "one", Icon: "heart", Heading: "Story", Body: "Hello"}}}
	if !validDesign(base, nil) {
		t.Fatal("valid design rejected")
	}
	base.Sections[0].PhotoKey = "other.jpg"
	if validDesign(base, nil) {
		t.Fatal("unowned image accepted")
	}
	base.Sections[0].PhotoKey = ""
	base.Sections[0].Icon = "<script>"
	if validDesign(base, nil) {
		t.Fatal("unsafe icon accepted")
	}
	base.Sections[0].Icon = "heart"
	base.AccentColor = "red; background:url(x)"
	if validDesign(base, nil) {
		t.Fatal("unsafe color accepted")
	}
}
