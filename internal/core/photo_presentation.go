package core

func validPhotoPresentation(el FlyerElement) bool {
	if el.Type != "image" {
		return el.PhotoTreatment == "" && el.PhotoFit == ""
	}
	if el.PhotoFit != "" && el.PhotoFit != "cover" && el.PhotoFit != "contain" {
		return false
	}
	switch el.PhotoTreatment {
	case "", "none", "soft", "fade", "arch", "cinema":
		return true
	}
	return false
}
