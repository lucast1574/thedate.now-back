package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"net/http"
	"time"
)

func (s *server) publishDesign(w http.ResponseWriter, r *http.Request) {
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo || e.PaymentStatus != "paid" || e.PublishedAt == nil {
		bad(w, 403, "Publica primero la invitación desde la organización del evento.")
		return
	}
	var in struct {
		ExpectedVersion *int64 `json:"expectedVersion"`
	}
	if decode(r, &in) != nil || in.ExpectedVersion == nil {
		bad(w, 400, "Revisa y guarda el borrador antes de publicar.")
		return
	}
	if *in.ExpectedVersion != e.DesignVersion {
		bad(w, 409, "El borrador cambió en otra sesión. Revísalo antes de publicar.")
		return
	}
	saved, err := (mongostore.Designs{DB: s.db}).Publish(r.Context(), e, time.Now().UTC())
	if err != nil {
		bad(w, 500, "No se pudieron publicar los cambios.")
		return
	}
	if !saved {
		bad(w, 409, "El borrador cambió mientras publicabas. Revísalo antes de publicar.")
		return
	}
	reply(w, 200, map[string]any{"status": "published", "publishedDesignVersion": e.DesignVersion})
}
