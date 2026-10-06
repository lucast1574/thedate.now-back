package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
)

func (s *server) getSeating(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 403, "Pay before using the seating manager")
		return
	}
	current, err := (mongostore.Responses{DB: s.db}).Snapshot(r.Context(), e.ID)
	if err != nil {
		bad(w, 500, "Could not load plan")
		return
	}
	plan := core.DefaultSeating()
	if current.Seating != nil {
		plan = *current.Seating
	}
	reply(w, 200, plan)
}
func (s *server) saveSeating(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 403, "Pay before using the seating manager")
		return
	}
	var plan core.SeatingPlan
	if decode(r, &plan) != nil {
		bad(w, 400, "Invalid seating plan")
		return
	}
	e, err = (mongostore.Responses{DB: s.db}).Snapshot(r.Context(), e.ID)
	if err != nil {
		bad(w, 500, "Could not load responses")
		return
	}
	cur, err := s.db.Collection("guests").Find(r.Context(), bson.M{"eventId": e.ID})
	if err != nil {
		bad(w, 500, "Could not load guests")
		return
	}
	defer cur.Close(r.Context())
	var guests []core.Guest
	if err = cur.All(r.Context(), &guests); err != nil {
		bad(w, 500, "Could not load guests")
		return
	}
	if err = core.ValidateSeating(plan, e, guests); err != nil {
		bad(w, 400, err.Error())
		return
	}
	saved, err := (mongostore.Seating{DB: s.db}).Save(r.Context(), e, plan)
	if err != nil {
		bad(w, 500, "Could not save seating plan")
		return
	}
	if !saved {
		bad(w, 409, "The guest responses or plan changed; reload before saving")
		return
	}
	plan.Version++
	reply(w, 200, plan)
}
