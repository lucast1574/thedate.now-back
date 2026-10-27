package core

func ValidRestrictions(el FlyerElement) bool {
	if el.Binding != "restrictions" {
		return el.RestrictionStyle == "" && el.RestrictionIcon == ""
	}
	switch el.RestrictionStyle {
	case "", "icon", "text", "icon-text":
	default:
		return false
	}
	return el.RestrictionIcon == "" || (el.RestrictionIcon != "none" && validFlyerIcon(el.RestrictionIcon))
}
