package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
)

func (s *server) inboxEvent(r *http.Request) (core.Event, error) { return s.planningEvent(r) }
func (s *server) listConversations(w http.ResponseWriter, r *http.Request) {
	e, err := s.inboxEvent(r)
	if err != nil || e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 404, "Evento no encontrado")
		return
	}
	list, err := (mongostore.Inbox{DB: s.db}).Conversations(r.Context(), e.ID)
	if err != nil {
		bad(w, 500, "No pudimos cargar los mensajes.")
		return
	}
	if r.URL.Query().Get("summary") == "unread" {
		unread := false
		for _, conversation := range list {
			if conversation.Unread {
				unread = true
				break
			}
		}
		reply(w, 200, map[string]bool{"unread": unread})
		return
	}
	reply(w, 200, list)
}
func (s *server) conversation(w http.ResponseWriter, r *http.Request) {
	e, err := s.inboxEvent(r)
	if err != nil || e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 404, "Evento no encontrado")
		return
	}
	guest := r.PathValue("guestId")
	var g core.Guest
	if s.db.Collection("guests").FindOne(r.Context(), bson.M{"_id": guest, "eventId": e.ID}).Decode(&g) != nil {
		bad(w, 404, "Invitado no encontrado")
		return
	}
	repo := mongostore.Inbox{DB: s.db}
	if r.Method == http.MethodPatch {
		var in struct {
			Through  string `json:"through"`
			Attended *bool  `json:"attended"`
		}
		if decode(r, &in) != nil || repo.Mark(r.Context(), e.ID, guest, in.Through, in.Attended) != nil {
			bad(w, 400, "No pudimos actualizar el estado del mensaje.")
			return
		}
		reply(w, 200, map[string]string{"status": "updated"})
		return
	}
	list, err := repo.Messages(r.Context(), e.ID, guest, r.URL.Query().Get("before"))
	if err != nil {
		bad(w, 400, "No pudimos cargar esta conversación.")
		return
	}
	u, _ := s.user(r)
	if e.Kind == "general" && e.Admission != nil && e.Admission.Enabled && (u.ID == e.OwnerID || u.Role == "admin") {
		if err := s.annotateReceipts(r.Context(), e, g, list); err != nil {
			bad(w, 500, "No pudimos cargar esta conversación.")
			return
		}
	}
	reply(w, 200, map[string]any{"messages": list, "hasMore": len(list) == 100})
}
