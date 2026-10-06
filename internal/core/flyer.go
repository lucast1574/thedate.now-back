package core

import "math"

type FlyerCanvas struct {
	Width      float64        `bson:"width" json:"width"`
	Height     float64        `bson:"height" json:"height"`
	Background string         `bson:"background" json:"background"`
	Elements   []FlyerElement `bson:"elements" json:"elements"`
}
type FlyerElement struct {
	Binding  string  `bson:"binding,omitempty" json:"binding,omitempty"`
	ID       string  `bson:"id" json:"id"`
	Type     string  `bson:"type" json:"type"`
	X        float64 `bson:"x" json:"x"`
	Y        float64 `bson:"y" json:"y"`
	Width    float64 `bson:"width" json:"width"`
	Height   float64 `bson:"height" json:"height"`
	Rotation float64 `bson:"rotation" json:"rotation"`
	Text     string  `bson:"text,omitempty" json:"text,omitempty"`
	PhotoKey string  `bson:"photoKey,omitempty" json:"photoKey,omitempty"`
	Icon     string  `bson:"icon,omitempty" json:"icon,omitempty"`
	Color    string  `bson:"color" json:"color"`
	Font     string  `bson:"font" json:"font"`
	FontSize float64 `bson:"fontSize" json:"fontSize"`
	Bold     bool    `bson:"bold" json:"bold"`
	Align    string  `bson:"align" json:"align"`
}

func bounded(n, min, max float64) bool {
	return !math.IsNaN(n) && !math.IsInf(n, 0) && n >= min && n <= max
}
func validPhoto(key string, photos []string) bool {
	for _, photo := range photos {
		if key == photo {
			return true
		}
	}
	return false
}
func ValidCanvas(c *FlyerCanvas, photos []string) bool {
	if c == nil {
		return true
	}
	if !bounded(c.Width, 320, 1200) || !bounded(c.Height, 240, 2000) || !validColor(c.Background) || len(c.Elements) > 60 {
		return false
	}
	seen := map[string]bool{}
	for _, el := range c.Elements {
		if el.ID == "" || len(el.ID) > 64 || seen[el.ID] || !bounded(el.X, 0, c.Width) || !bounded(el.Y, 0, c.Height) || !bounded(el.Width, 16, c.Width) || !bounded(el.Height, 16, c.Height) || el.X+el.Width > c.Width+0.01 || el.Y+el.Height > c.Height+0.01 || !bounded(el.Rotation, -180, 180) || !bounded(el.FontSize, 8, 160) || !validColor(el.Color) {
			return false
		}
		seen[el.ID] = true
		if el.Binding != "" && (el.Type != "text" || el.Binding != "guest_name") {
			return false
		}
		if el.Font != "serif" && el.Font != "sans" && el.Font != "mono" && el.Font != "script" {
			return false
		}
		if el.Align != "left" && el.Align != "center" && el.Align != "right" {
			return false
		}
		switch el.Type {
		case "text":
			if len(el.Text) > 2000 || el.PhotoKey != "" || el.Icon != "" {
				return false
			}
		case "image":
			if !validPhoto(el.PhotoKey, photos) || el.Text != "" || el.Icon != "" {
				return false
			}
		case "icon":
			if !sectionIcons[el.Icon] || el.Icon == "none" || el.Text != "" || el.PhotoKey != "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}
