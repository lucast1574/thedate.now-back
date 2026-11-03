package core

func validStudioPresentation(el FlyerElement) bool {
	if el.Typeface != "" && el.Typeface != "display" && el.Typeface != "rounded" && el.Typeface != "humanist" {
		return false
	}
	if el.Type != "text" && (el.Typeface != "" || el.Italic || el.LineHeight != nil || el.LetterSpacing != nil) {
		return false
	}
	if el.Type != "image" && (el.PhotoPositionX != nil || el.PhotoPositionY != nil) {
		return false
	}
	for _, field := range []struct {
		value    *float64
		min, max float64
	}{
		{el.LineHeight, 0.8, 2.4}, {el.LetterSpacing, -0.08, 0.3},
		{el.PhotoPositionX, 0, 100}, {el.PhotoPositionY, 0, 100}, {el.Opacity, 0.05, 1},
	} {
		if field.value != nil && !bounded(*field.value, field.min, field.max) {
			return false
		}
	}
	return true
}
