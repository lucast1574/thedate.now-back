package core

import "strings"

var sectionIcons = map[string]bool{"heart": true, "sparkle": true, "flower": true, "ring": true, "music": true, "star": true, "calendar": true, "none": true}

type DesignInput struct {
	TemplateID  string    `json:"templateId"`
	DesignMode  string    `json:"designMode"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Template    string    `json:"template"`
	AccentColor string    `json:"accentColor"`
	Sections    []Section `json:"sections"`
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
