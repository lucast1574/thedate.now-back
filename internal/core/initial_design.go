package core

// Initial designs are validated before the event's single insert. Metadata updates
// cannot use this path to replace an invitation that has already been edited.
func ValidInitialDesign(in EventInput, creating bool) bool {
	d := in.InitialDesign
	if d == nil {
		return true
	}
	if !creating || d.DesignMode != "flyer" || d.DesignMode != in.DesignMode || d.TemplateID != in.TemplateID || d.Template != in.Template || d.AccentColor != in.AccentColor || d.Title != in.Title || d.Description != in.Description || d.Music != nil || d.BackgroundImage != nil {
		return false
	}
	if (d.Celebration == nil) != (in.Celebration == nil) || (d.Celebration != nil && *d.Celebration != *in.Celebration) {
		return false
	}
	return len(d.Sections) > 0 && ValidDesign(*d, nil)
}
