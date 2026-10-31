package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
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
	if decode(r, &in) != nil || !validDesign(in, e.PhotoKeys) || !core.ValidMusic(in.Music, e.AudioKeys) || core.TemplateID(e.Kind, in.Template, in.DesignMode, in.TemplateID) == "" {
		bad(w, 400, "Invalid invitation design")
		return
	}
	if err := core.ValidateProductContent(e.Kind, in.Title, in.Celebration, in.Sections); err != nil {
		bad(w, 400, err.Error())
		return
	}
	if (e.PublishedAt != nil && in.ExpectedVersion == nil) || (in.ExpectedVersion != nil && *in.ExpectedVersion != e.DesignVersion) {
		bad(w, 409, "El diseño cambió en otra sesión. Vuelve a abrir el editor para revisar la versión actual; tu copia local se conserva.")
		return
	}
	next := core.EditedDesign(e, in)
	if core.SameDesign(e, next) {
		reply(w, 200, map[string]any{"status": "unchanged", "designVersion": e.DesignVersion})
		return
	}
	saved, err := (mongostore.Designs{DB: s.db}).Save(r.Context(), e, next, time.Now().UTC())
	if err != nil {
		bad(w, 500, "Could not save design")
		return
	}
	if !saved {
		bad(w, 409, "El diseño cambió mientras guardabas. Tu copia local se conserva; vuelve a abrir el editor.")
		return
	}
	reply(w, 200, map[string]any{"status": "updated", "designVersion": e.DesignVersion + 1})
}

func (s *server) ensureDemos(ctx context.Context, u core.User) error {
	kinds := []string{}
	if core.UserCanCreate("wedding", u) {
		kinds = append(kinds, "wedding")
	}
	if core.UserCanCreate("general", u) {
		kinds = append(kinds, "general")
	}
	for _, kind := range kinds {
		if s.product != "" && kind != s.product {
			continue
		}
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
