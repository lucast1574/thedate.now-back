package core

import (
	"embed"
	"encoding/json"
)

//go:embed templates.json
var templateFiles embed.FS

type InvitationTemplate struct {
	ID         string `bson:"_id" json:"id"`
	Kind       string `bson:"kind" json:"kind"`
	Mode       string `bson:"mode" json:"mode"`
	Name       string `bson:"name" json:"name"`
	Accent     string `bson:"accent" json:"accent"`
	Background string `bson:"background" json:"background"`
	Font       string `bson:"font" json:"font"`
	Style      string `bson:"style" json:"style"`
	Tagline    string `bson:"tagline" json:"tagline"`
}

func TemplateCatalog() []InvitationTemplate {
	raw, _ := templateFiles.ReadFile("templates.json")
	var list []InvitationTemplate
	if err := json.Unmarshal(raw, &list); err != nil {
		panic("invalid built-in template catalog")
	}
	return list
}
func TemplateID(kind, style, mode, id string) string {
	if mode == "" {
		mode = "sections"
	}
	for _, t := range TemplateCatalog() {
		if t.ID == id && t.Kind == kind && t.Style == style && t.Mode == mode {
			return id
		}
	}
	if id != "" {
		return ""
	}
	for _, t := range TemplateCatalog() {
		if t.Kind == kind && t.Style == style && t.Mode == mode {
			return t.ID
		}
	}
	return ""
}
