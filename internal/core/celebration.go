package core

import "strings"

type Opening struct {
	Enabled     bool   `bson:"enabled" json:"enabled"`
	Style       string `bson:"style" json:"style"`
	Color       string `bson:"color" json:"color"`
	Ink         string `bson:"ink" json:"ink"`
	Message     string `bson:"message" json:"message"`
	ButtonLabel string `bson:"buttonLabel" json:"buttonLabel"`
}
type Celebration struct {
	AdultsOnly     bool   `bson:"adultsOnly" json:"adultsOnly"`
	AdultsMessage  string `bson:"adultsMessage" json:"adultsMessage"`
	WhatsAppNotice bool   `bson:"whatsappNotice" json:"whatsappNotice"`
	DressCode      string `bson:"dressCode" json:"dressCode"`
	DressNote      string `bson:"dressNote" json:"dressNote"`
}

func ValidCelebration(c *Celebration) bool {
	return c == nil || (len(c.AdultsMessage) <= 500 && len(c.DressCode) <= 120 && len(c.DressNote) <= 500)
}
func ValidOpening(o *Opening) bool {
	return o == nil || ((o.Style == "classic" || o.Style == "arch" || o.Style == "ticket") && validColor(o.Color) && validColor(o.Ink) && len(o.Message) <= 240 && len(strings.TrimSpace(o.ButtonLabel)) > 0 && len(o.ButtonLabel) <= 60)
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
