package httpapi

import (
	"errors"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"net/http"
	"strings"
	"time"
)

func (s *server) createEvent(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	var in eventInput
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid request")
		return
	}
	if err = core.ValidateEvent(in, true); err != nil {
		bad(w, 400, "Invalid event details")
		return
	}
	if !core.UserCanCreate(in.Kind, u) {
		bad(w, 403, "Account cannot create this event kind")
		return
	}
	in = core.NormalizeEvent(in)
	if in.MaybeHoldHours == 0 {
		in.MaybeHoldHours = 48
	}
	if in.MaybeHoldHours < 1 || in.MaybeHoldHours > 168 {
		bad(w, 400, "Maybe hold must be between 1 and 168 hours")
		return
	}
	now := time.Now().UTC()
	status := "unpaid"
	if u.Role == "admin" {
		status = "paid"
	}
	e := core.Event{ID: uuid.NewString(), Kind: in.Kind, Slug: in.Slug, Title: strings.TrimSpace(in.Title), Description: in.Description, StartAt: in.StartAt, TimeZone: in.TimeZone, Organizer: strings.TrimSpace(in.Organizer), Location: eventLocation(in), IsVirtual: in.IsVirtual, MapURL: physicalMapURL(in), VirtualURL: virtualEventURL(in), Capacity: in.Capacity, CapacityUnlimited: in.CapacityUnlimited, MaybeHoldHours: in.MaybeHoldHours, Template: in.Template, TemplateID: in.TemplateID, DesignMode: in.DesignMode, AccentColor: in.AccentColor, OwnerID: u.ID, PaymentStatus: status, PaymentSource: func() string {
		if u.Role == "admin" {
			return "courtesy"
		}
		return ""
	}(), CreatedAt: now, UpdatedAt: now, PhotoKeys: []string{}, Sections: []core.Section{}, CoupleUserIDs: []string{}}
	if u.Role == "admin" && s.audit(r, u, "courtesy-create", e.ID, "Administrator own event") != nil {
		bad(w, 500, "Could not audit courtesy")
		return
	}
	if _, err = s.db.Collection("events").InsertOne(r.Context(), e); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			bad(w, 409, "That invitation address is already taken")
		} else {
			bad(w, 500, "Could not create event")
		}
		return
	}
	reply(w, 201, e)
}

func (s *server) listEvents(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	if err := s.ensureDemos(r.Context(), u); err != nil {
		bad(w, 500, "Could not prepare demo invitation")
		return
	}
	filter := bson.M{"$or": bson.A{bson.M{"ownerId": u.ID}, bson.M{"coupleUserIds": u.ID}}}
	if u.Role == "admin" {
		filter = bson.M{"$or": bson.A{bson.M{"isDemo": bson.M{"$ne": true}}, bson.M{"ownerId": u.ID}}}
	}
	cur, err := s.db.Collection("events").Find(r.Context(), filter)
	if err != nil {
		bad(w, 500, "Could not list events")
		return
	}
	defer cur.Close(r.Context())
	events := []core.Event{}
	if err = cur.All(r.Context(), &events); err != nil {
		bad(w, 500, "Could not list events")
		return
	}
	for i := range events {
		s.completeLegacyEvent(r.Context(), &events[i])
	}
	reply(w, 200, events)
}

func (s *server) ownedEvent(r *http.Request) (core.Event, error) {
	var e core.Event
	u, err := s.user(r)
	if err != nil {
		return e, err
	}
	filter := bson.M{"_id": r.PathValue("id"), "$or": bson.A{bson.M{"ownerId": u.ID}, bson.M{"coupleUserIds": u.ID}}}
	if u.Role == "admin" {
		filter = bson.M{"_id": r.PathValue("id")}
	}
	err = s.db.Collection("events").FindOne(r.Context(), filter).Decode(&e)
	return e, err
}

// Couples assigned to a wedding may manage its guest list and seating plan.
func (s *server) planningEvent(r *http.Request) (core.Event, error) {
	return s.ownedEvent(r)
}

func (s *server) managedEvent(r *http.Request) (core.Event, error) {
	e, err := s.ownedEvent(r)
	if err != nil {
		return e, err
	}
	u, err := s.user(r)
	if err != nil {
		return e, err
	}
	if u.Role != "admin" && e.OwnerID != u.ID {
		return e, errors.New("manager access required")
	}
	return e, nil
}

func (s *server) updateEvent(w http.ResponseWriter, r *http.Request) {
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo {
		bad(w, 403, "Use the design editor for demo invitations")
		return
	}
	var in eventInput
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid request")
		return
	}
	if in.Kind != e.Kind || core.ValidateEvent(in, false) != nil || (e.PublishedAt != nil && in.Slug != e.Slug) {
		bad(w, 400, "Invalid event details")
		return
	}
	hold := in.MaybeHoldHours
	if hold == 0 {
		hold = e.MaybeHoldHours
	}
	if hold < 1 || hold > 168 {
		bad(w, 400, "Invalid hold period")
		return
	}
	in.MaybeHoldHours = hold
	err = s.responses().UpdateEvent(r.Context(), e.ID, in)
	if err != nil {
		if errors.Is(err, errCapacity) {
			bad(w, 409, "Capacity cannot be lower than active reservations")
		} else {
			bad(w, 500, "Could not update event")
		}
		return
	}
	reply(w, 200, map[string]string{"status": "updated"})
}

func (s *server) publicEvent(w http.ResponseWriter, r *http.Request) {
	kind, slug := r.PathValue("kind"), r.PathValue("slug")
	if (kind != "wedding" && kind != "general") || core.ValidateSlug(slug) != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	var e core.Event
	err := s.db.Collection("events").FindOne(r.Context(), bson.M{"kind": kind, "slug": slug, "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}}).Decode(&e)
	if err != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	s.completeLegacyEvent(r.Context(), &e)
	reply(w, 200, publicEventView(e))
}
