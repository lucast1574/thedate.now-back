package main

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

var sectionIcons = map[string]bool{"heart": true, "sparkle": true, "flower": true, "ring": true, "music": true, "star": true, "calendar": true, "none": true}

type designInput struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Template    string         `json:"template"`
	AccentColor string         `json:"accentColor"`
	Sections    []core.Section `json:"sections"`
}

func validDesign(in designInput, photos []string) bool {
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

func (s *server) updateDesign(w http.ResponseWriter, r *http.Request) {
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	var in designInput
	if decode(r, &in) != nil || !validDesign(in, e.PhotoKeys) {
		bad(w, 400, "Invalid invitation design")
		return
	}
	_, err = s.db.Collection("events").UpdateByID(r.Context(), e.ID, bson.M{"$set": bson.M{"title": strings.TrimSpace(in.Title), "description": in.Description, "template": in.Template, "accentColor": in.AccentColor, "sections": in.Sections, "updatedAt": time.Now().UTC()}})
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
	if u.Role == "planner" || u.Role == "admin" {
		kinds = append(kinds, "wedding")
	}
	if u.Role == "organizer" || u.Role == "admin" {
		kinds = append(kinds, "general")
	}
	for _, kind := range kinds {
		id := "demo-" + kind + "-" + u.ID
		now := time.Now().UTC()
		title := "Una celebración inolvidable"
		if kind == "wedding" {
			title = "Nuestra historia"
		}
		e := core.Event{ID: id, Kind: kind, Slug: "demo-" + uuid.NewString(), IsDemo: true, Title: title, Description: "Edita esta invitación de muestra para probar tu estilo.", StartAt: now.AddDate(0, 3, 0), TimeZone: "America/Bogota", Organizer: u.Name, Location: "Lugar por definir", Capacity: 50, MaybeHoldHours: 48, Template: "classic", AccentColor: "#ad7254", PhotoKeys: []string{}, Sections: []core.Section{}, CoupleUserIDs: []string{}, OwnerID: u.ID, PaymentStatus: "demo", CreatedAt: now, UpdatedAt: now}
		if _, err := s.db.Collection("events").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$setOnInsert": e}, options.UpdateOne().SetUpsert(true)); err != nil {
			return err
		}
	}
	return nil
}
