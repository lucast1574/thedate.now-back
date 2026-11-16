package httpapi

import (
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"net/http"
)

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
	if err := core.ValidateProductContent(e.Kind, in.Title, in.Celebration, e.Sections); err != nil {
		bad(w, 400, err.Error())
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
		if errors.Is(err, core.ErrAdmissionLocked) {
			bad(w, 409, err.Error())
		} else if errors.Is(err, core.ErrEventClosed) {
			bad(w, 409, "Esta acción no está disponible.")
		} else if errors.Is(err, errCapacity) {
			bad(w, 409, "Capacity cannot be lower than active reservations")
		} else if errors.Is(err, core.ErrPublishedAddress) {
			bad(w, 409, err.Error())
		} else if mongo.IsDuplicateKeyError(err) {
			bad(w, 409, "Ese enlace ya está en uso. Elige otro.")
		} else {
			bad(w, 500, "Could not update event")
		}
		return
	}
	reply(w, 200, map[string]string{"status": "updated"})
}
