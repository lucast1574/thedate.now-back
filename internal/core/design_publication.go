package core

import (
	"encoding/json"
	"strings"
)

// The snapshot contains visual content and its media library, never RSVP or payment state.
type DesignSnapshot struct {
	DesignInput `bson:",inline"`
	PhotoKeys   []string `bson:"photoKeys" json:"photoKeys"`
	AudioKeys   []string `bson:"audioKeys" json:"audioKeys"`
}

func SnapshotDesign(e Event) DesignSnapshot {
	return DesignSnapshot{DesignInput: DesignInput{
		BackgroundImage: e.BackgroundImage, Opening: e.Opening, Celebration: e.Celebration,
		Music: e.Music, TemplateID: e.TemplateID, DesignMode: e.DesignMode, Title: e.Title,
		Description: e.Description, Template: e.Template, AccentColor: e.AccentColor, Sections: e.Sections,
	}, PhotoKeys: e.PhotoKeys, AudioKeys: e.AudioKeys}
}

func ApplyDesign(e Event, d DesignInput) Event {
	e.BackgroundImage, e.Opening, e.Celebration, e.Music = d.BackgroundImage, d.Opening, d.Celebration, d.Music
	e.Title, e.Description, e.Template, e.TemplateID = d.Title, d.Description, d.Template, d.TemplateID
	e.AccentColor, e.DesignMode, e.Sections = d.AccentColor, d.DesignMode, d.Sections
	return e
}

func EditedDesign(e Event, in DesignInput) Event {
	if in.BackgroundImage == nil && !in.ClearBackgroundImage {
		in.BackgroundImage = e.BackgroundImage
	}
	if in.Opening == nil {
		in.Opening = e.Opening
	}
	if in.Celebration == nil {
		in.Celebration = e.Celebration
	}
	in.Title = strings.TrimSpace(in.Title)
	in.TemplateID = TemplateID(e.Kind, in.Template, in.DesignMode, in.TemplateID)
	return ApplyDesign(e, in)
}

func PublishedEvent(e Event) Event {
	if e.PublishedDesign == nil {
		return e
	}
	d := e.PublishedDesign
	e = ApplyDesign(e, d.DesignInput)
	e.PhotoKeys, e.AudioKeys = d.PhotoKeys, d.AudioKeys
	return e
}

func SameDesign(a, b Event) bool {
	left, _ := json.Marshal(SnapshotDesign(a).DesignInput)
	right, _ := json.Marshal(SnapshotDesign(b).DesignInput)
	return string(left) == string(right)
}
