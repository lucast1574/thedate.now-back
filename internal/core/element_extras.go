package core

func ValidElementExtras(el FlyerElement) bool {
	if el.NameFormat != "" && (el.Binding != "couple_names" || (el.NameFormat != "names" && el.NameFormat != "initials" && el.NameFormat != "stacked" && el.NameFormat != "uppercase")) {
		return false
	}
	if len(el.HintText) > 160 || (el.HintStyle != "" && el.HintStyle != "text" && el.HintStyle != "icon" && el.HintStyle != "icon-text") {
		return false
	}
	if el.HintIcon != "" && el.HintIcon != "external" && el.HintIcon != "arrow" && !validFlyerIcon(el.HintIcon) {
		return false
	}
	if el.ActionHint && el.Binding != "map" && el.Binding != "location" && el.Binding != "gift_link" {
		return false
	}
	if (el.MapLatitude == nil) != (el.MapLongitude == nil) {
		return false
	}
	if el.MapLatitude != nil && (!bounded(*el.MapLatitude, -85, 85) || !bounded(*el.MapLongitude, -180, 180)) {
		return false
	}
	if el.Binding != "map" {
		return el.MapLatitude == nil && el.MapZoom == 0 && el.MapStyle == "" && el.MapRadius == 0 && el.MapSoftness == 0
	}
	return (el.MapZoom == 0 || bounded(el.MapZoom, 3, 18)) && bounded(el.MapRadius, 0, 80) && bounded(el.MapSoftness, 0, 35) && (el.MapStyle == "" || el.MapStyle == "natural" || el.MapStyle == "warm" || el.MapStyle == "ink")
}
