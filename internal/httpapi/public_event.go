package httpapi

import "github.com/lucast1574/thedate.now-back/internal/core"

// Explicit allowlist: public invitations never serialize internal account/payment state.
func publicEventView(e core.Event) map[string]any {
	return map[string]any{"id": e.ID, "kind": e.Kind, "slug": e.Slug, "title": e.Title, "description": e.Description, "startAt": e.StartAt, "timeZone": e.TimeZone, "organizer": e.Organizer, "location": e.Location, "isVirtual": e.IsVirtual, "mapUrl": e.MapURL, "virtualUrl": e.VirtualURL, "template": e.Template, "templateId": core.TemplateID(e.Kind, e.Template, e.DesignMode, e.TemplateID), "accentColor": e.AccentColor, "sections": e.Sections, "designMode": e.DesignMode, "photoKeys": e.PhotoKeys, "publishedAt": e.PublishedAt}
}
