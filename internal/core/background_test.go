package core

import (
	"math"
	"testing"
)

func TestBackgroundRequiresOwnedPhotoAndBoundedPresentation(t *testing.T) {
	bg := &ImageBackground{PhotoKey: "mine.jpg", Fit: "cover", Position: "center", Opacity: .5}
	if !ValidBackground(bg, []string{"mine.jpg"}) {
		t.Fatal("owned background rejected")
	}
	if ValidBackground(bg, nil) {
		t.Fatal("foreign photo accepted")
	}
	for _, key := range []string{"template-photo", "mine.jpg"} {
		bg.PhotoKey = key
		for _, fit := range []string{"cover", "contain"} {
			for _, pos := range []string{"center", "top", "bottom"} {
				bg.Fit = fit
				bg.Position = pos
				if !ValidBackground(bg, []string{"mine.jpg"}) {
					t.Fatal("valid presentation rejected")
				}
			}
		}
	}
	for _, opacity := range []float64{-1, 1.1, math.NaN(), math.Inf(1)} {
		bg.Opacity = opacity
		if ValidBackground(bg, []string{"mine.jpg"}) {
			t.Fatal("invalid opacity accepted")
		}
	}
	bg.Opacity = 1
	bg.Fit = "url(evil)"
	if ValidBackground(bg, []string{"mine.jpg"}) {
		t.Fatal("invalid fit accepted")
	}
	bg.Fit = "cover"
	bg.Position = "left"
	if ValidBackground(bg, []string{"mine.jpg"}) {
		t.Fatal("invalid position accepted")
	}
}
