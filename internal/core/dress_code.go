package core

import (
	"strings"
	"unicode/utf8"
)

type DressExcludedColor struct {
	Name  string `bson:"name" json:"name"`
	Color string `bson:"color" json:"color"`
}

func ValidDressCode(el FlyerElement, photos []string) bool {
	if el.Binding != "dress_code" {
		return el.DressPaletteStyle == "" && el.DressStyle == "" && el.DressIcon == "" && el.DressPhotoKey == "" && len(el.DressExcludedColors) == 0
	}
	switch el.DressPaletteStyle {
	case "", "fabric", "petal", "bar", "jewel", "wave", "ticket":
	default:
		return false
	}
	switch el.DressStyle {
	case "", "text", "icon", "image", "icon-text", "image-text", "icon-image", "all":
	default:
		return false
	}
	if el.DressIcon != "" && (el.DressIcon == "none" || !validFlyerIcon(el.DressIcon)) {
		return false
	}
	if len(el.DressExcludedColors) > 8 {
		return false
	}
	for _, rule := range el.DressExcludedColors {
		if strings.TrimSpace(rule.Name) == "" || utf8.RuneCountInString(rule.Name) > 40 || !validColor(rule.Color) {
			return false
		}
	}
	return el.DressPhotoKey == "" || validPhoto(el.DressPhotoKey, photos)
}
