package core

import "strings"

type Opening struct {
	BackgroundImage *ImageBackground `bson:"backgroundImage,omitempty" json:"backgroundImage,omitempty"`
	Title           string           `bson:"title,omitempty" json:"title,omitempty"`
	Font            string           `bson:"font,omitempty" json:"font,omitempty"`
	SealIcon        string           `bson:"sealIcon,omitempty" json:"sealIcon,omitempty"`
	PaletteMode     string           `bson:"paletteMode,omitempty" json:"paletteMode,omitempty"`
	Enabled         bool             `bson:"enabled" json:"enabled"`
	Style           string           `bson:"style" json:"style"`
	Color           string           `bson:"color" json:"color"`
	Ink             string           `bson:"ink" json:"ink"`
	Message         string           `bson:"message" json:"message"`
	ButtonLabel     string           `bson:"buttonLabel" json:"buttonLabel"`
}
type Celebration struct {
	DressExcludedColors []DressExcludedColor `bson:"dressExcludedColors,omitempty" json:"dressExcludedColors,omitempty"`
	AdultsOnly          bool                 `bson:"adultsOnly" json:"adultsOnly"`
	AdultsMessage       string               `bson:"adultsMessage" json:"adultsMessage"`
	WhatsAppNotice      bool                 `bson:"whatsappNotice" json:"whatsappNotice"`
	DressCode           string               `bson:"dressCode" json:"dressCode"`
	DressNote           string               `bson:"dressNote" json:"dressNote"`
}

func ValidCelebration(c *Celebration) bool {
	return c == nil || (len(c.AdultsMessage) <= 500 && len(c.DressCode) <= 120 && len(c.DressNote) <= 500 && ValidDressCode(FlyerElement{Binding: "dress_code", DressExcludedColors: c.DressExcludedColors}, nil))
}
func ValidOpening(o *Opening) bool {
	if o == nil {
		return true
	}
	if o.PaletteMode != "" && o.PaletteMode != "design" && o.PaletteMode != "custom" {
		return false
	}
	if len(o.Title) > 160 || (o.Font != "" && o.Font != "serif" && o.Font != "sans" && o.Font != "script") || (o.SealIcon != "" && !validFlyerIcon(o.SealIcon)) {
		return false
	}
	validStyle := o.Style == "classic" || o.Style == "arch" || o.Style == "ticket" || o.Style == "curtain" || o.Style == "bloom" || o.Style == "orbit"
	return validStyle && validColor(o.Color) && validColor(o.Ink) && len(o.Message) <= 240 && len(strings.TrimSpace(o.ButtonLabel)) > 0 && len(o.ButtonLabel) <= 60
}

func WhatsAppCelebration(c *Celebration) string {
	if c == nil || !c.AdultsOnly || !c.WhatsAppNotice {
		return ""
	}
	message := strings.TrimSpace(c.AdultsMessage)
	if message == "" {
		message = "Celebración solo para adultos. Gracias por tu comprensión."
	}
	return "\n\n" + message
}
