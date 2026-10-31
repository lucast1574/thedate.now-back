package core

import "strings"

var sectionIcons = map[string]bool{"heart": true, "sparkle": true, "flower": true, "ring": true, "music": true, "star": true, "calendar": true, "none": true, "frame": true, "arch": true, "leaf": true, "sunburst": true}

type DesignInput struct {
	ExpectedVersion      *int64           `bson:"-" json:"expectedVersion,omitempty"`
	BackgroundImage      *ImageBackground `bson:"backgroundImage" json:"backgroundImage"`
	ClearBackgroundImage bool             `bson:"-" json:"clearBackgroundImage"`
	Opening              *Opening         `bson:"opening" json:"opening"`
	Celebration          *Celebration     `bson:"celebration" json:"celebration"`
	Music                *Music           `bson:"music" json:"music"`
	TemplateID           string           `bson:"templateId" json:"templateId"`
	DesignMode           string           `bson:"designMode" json:"designMode"`
	Title                string           `bson:"title" json:"title"`
	Description          string           `bson:"description" json:"description"`
	Template             string           `bson:"template" json:"template"`
	AccentColor          string           `bson:"accentColor" json:"accentColor"`
	Sections             []Section        `bson:"sections" json:"sections"`
}

func validColor(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	for _, c := range color[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
func ValidDesign(in DesignInput, photos []string) bool {
	if in.Opening != nil && !ValidBackground(in.Opening.BackgroundImage, photos) {
		return false
	}
	if !ValidBackground(in.BackgroundImage, photos) || !ValidOpening(in.Opening) || !ValidCelebration(in.Celebration) {
		return false
	}
	if in.DesignMode != "" && in.DesignMode != "sections" && in.DesignMode != "flyer" {
		return false
	}
	if len(strings.TrimSpace(in.Title)) < 2 || len(in.Title) > 160 || len(in.Description) > 5000 || (in.Template != "classic" && in.Template != "modern") || len(in.AccentColor) != 7 || in.AccentColor[0] != '#' || len(in.Sections) > 20 {
		return false
	}
	for _, c := range in.AccentColor[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	seen := map[string]bool{}
	for _, section := range in.Sections {
		if section.When != "" && section.When != "gifts" && section.When != "restrictions" {
			return false
		}
		if len(section.ID) > 64 || section.ID == "" || seen[section.ID] || !sectionIcons[section.Icon] || len(section.Heading) > 120 || len(section.Body) > 2000 {
			return false
		}
		seen[section.ID] = true
		if section.GuestText != nil {
			text := section.GuestText
			if text.Type != "text" || text.Binding != "guest_name" || !ValidCanvas(&FlyerCanvas{Width: 720, Height: 240, Background: "#ffffff", Elements: []FlyerElement{*text}}, nil) {
				return false
			}
		}
		if !ValidCanvas(section.Canvas, photos) || (in.DesignMode == "flyer" && section.Canvas == nil) {
			return false
		}
		if section.PhotoKey != "" {
			found := false
			for _, key := range photos {
				if key == section.PhotoKey {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}
