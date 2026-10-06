package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type designInput = core.DesignInput

var validDesign = core.ValidDesign

func (s *server) updateDesign(w http.ResponseWriter, r *http.Request) {
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	var in designInput
	if decode(r, &in) != nil || !validDesign(in, e.PhotoKeys) || core.TemplateID(e.Kind, in.Template, in.DesignMode, in.TemplateID) == "" {
		bad(w, 400, "Invalid invitation design")
		return
	}
	_, err = s.db.Collection("events").UpdateByID(r.Context(), e.ID, bson.M{"$set": bson.M{"title": strings.TrimSpace(in.Title), "description": in.Description, "template": in.Template, "templateId": core.TemplateID(e.Kind, in.Template, in.DesignMode, in.TemplateID), "accentColor": in.AccentColor, "sections": in.Sections, "designMode": in.DesignMode, "updatedAt": time.Now().UTC()}})
	if err != nil {
		bad(w, 500, "Could not save design")
		return
	}
	reply(w, 200, map[string]string{"status": "updated"})
}

func (s *server) ensureDemos(ctx context.Context, u core.User) error {
	if u.Role == "couple" {
		return nil
	}
	kinds := []string{}
	if core.UserCanCreate("wedding", u) {
		kinds = append(kinds, "wedding")
	}
	if core.UserCanCreate("general", u) {
		kinds = append(kinds, "general")
	}
	for _, kind := range kinds {
		id := "demo-" + kind + "-" + u.ID
		now := time.Now().UTC()
		title := "Una celebración inolvidable"
		if kind == "wedding" {
			title = "Nuestra historia"
		}
		e := core.Event{ID: id, Kind: kind, Slug: "demo-" + uuid.NewString(), IsDemo: true, Title: title, Description: "Edita esta invitación de muestra para probar tu estilo.", StartAt: now.AddDate(0, 3, 0), TimeZone: "America/Bogota", Organizer: u.Name, Location: "Lugar por definir", Capacity: 50, MaybeHoldHours: 48, Template: "classic", TemplateID: core.TemplateID(kind, "classic", "sections", ""), AccentColor: "#ad7254", PhotoKeys: []string{}, Sections: []core.Section{}, CoupleUserIDs: []string{}, OwnerID: u.ID, PaymentStatus: "demo", CreatedAt: now, UpdatedAt: now}
		if _, err := s.db.Collection("events").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$setOnInsert": e}, options.UpdateOne().SetUpsert(true)); err != nil {
			return err
		}
	}
	return nil
}
