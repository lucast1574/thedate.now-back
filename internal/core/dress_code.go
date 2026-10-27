package core

func ValidDressCode(el FlyerElement, photos []string) bool {
	if el.Binding != "dress_code" {
		return el.DressStyle == "" && el.DressIcon == "" && el.DressPhotoKey == ""
	}
	switch el.DressStyle {
	case "", "text", "icon", "image", "icon-text", "image-text", "icon-image", "all":
	default:
		return false
	}
	if el.DressIcon != "" && (el.DressIcon == "none" || !validFlyerIcon(el.DressIcon)) {
		return false
	}
	return el.DressPhotoKey == "" || validPhoto(el.DressPhotoKey, photos)
}
