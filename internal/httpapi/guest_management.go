package httpapi

import (
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"strings"
	"time"
)

func (s *server) manageGuest(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil || e.PaymentStatus != "paid" || e.IsDemo {
		bad(w, 404, "Evento no encontrado")
		return
	}
	var g core.Guest
	if s.db.Collection("guests").FindOne(r.Context(), bson.M{"_id": r.PathValue("guestId"), "eventId": e.ID}).Decode(&g) != nil {
		bad(w, 404, "Invitado no encontrado")
		return
	}
	if r.Method == http.MethodDelete {
		err = (mongostore.GuestManagement{Responses: mongostore.Responses{DB: s.db}}).Remove(r.Context(), e.ID, g.ID, time.Now().UTC())
	} else {
		var in struct {
			Response string `json:"response"`
			Reason   string `json:"reason"`
		}
		if decode(r, &in) != nil || (in.Response != "going" && in.Response != "not_going" && in.Response != "maybe") || len(in.Reason) > 2000 || (in.Response == "maybe" && len(strings.TrimSpace(in.Reason)) < 3) {
			bad(w, 400, "Elige una respuesta y añade un motivo si es Tal vez.")
			return
		}
		err = (application.ManagedResponses{Repository: mongostore.Responses{DB: s.db}, Now: time.Now}).Correct(r.Context(), e.ID, g, in.Response, in.Reason)
	}
	if err != nil {
		if errors.Is(err, core.ErrCapacity) {
			bad(w, 409, "No hay cupos disponibles para confirmar este grupo.")
			return
		}
		if errors.Is(err, mongostore.ErrGuestSending) || errors.Is(err, mongostore.ErrReceivedGift) {
			bad(w, 409, err.Error())
			return
		}
		if errors.Is(err, core.ErrUnavailable) {
			bad(w, 409, "La invitación ya no está disponible. Recarga la lista.")
			return
		}
		bad(w, 500, "No pudimos actualizar al invitado. Vuelve a intentarlo.")
		return
	}
	reply(w, 200, map[string]string{"status": "updated"})
}
