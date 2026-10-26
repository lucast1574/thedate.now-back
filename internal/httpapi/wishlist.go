package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/linkpreview"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
)

func (s *server) getWishlist(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Evento no encontrado")
		return
	}
	names := map[string]string{}
	if cur, err := s.db.Collection("guests").Find(r.Context(), bson.M{"eventId": e.ID}); err == nil {
		defer cur.Close(r.Context())
		var guests []core.Guest
		if cur.All(r.Context(), &guests) != nil {
			bad(w, 500, "No pudimos cargar los invitados")
			return
		}
		for _, g := range guests {
			names[g.ID] = core.GuestFullName(g)
		}
	} else {
		bad(w, 500, "No pudimos cargar los invitados")
		return
	}
	if e.Wishlist.Items == nil {
		e.Wishlist.Items = []core.Gift{}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	reply(w, 200, map[string]any{"list": e.Wishlist, "guests": names})
}
func (s *server) saveWishlist(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Evento no encontrado")
		return
	}
	var in core.Wishlist
	if decode(r, &in) != nil || !core.ValidWishlist(in) {
		bad(w, 400, "Revisa los datos de la lista (máximo 100 regalos).")
		return
	}
	for _, item := range in.Items {
		if !linkpreview.ValidURL(item.URL) || (item.ImageURL != "" && !linkpreview.ValidURL(item.ImageURL)) {
			bad(w, 400, "Usa enlaces HTTPS públicos para el artículo y la imagen.")
			return
		}
	}
	if in.Version != e.Wishlist.Version {
		bad(w, 409, "La lista cambió. Recarga antes de guardar; conservamos tu borrador.")
		return
	}
	next, err := core.EditWishlist(e.Wishlist, in)
	if err != nil {
		bad(w, 409, err.Error())
		return
	}
	saved, err := (mongostore.Wishlists{DB: s.db}).Commit(r.Context(), e.ID, e.Wishlist.Version, next, false)
	if err != nil {
		bad(w, 500, "No pudimos guardar la lista.")
		return
	}
	if !saved {
		bad(w, 409, "Hay nuevas reservas. Recarga antes de guardar.")
		return
	}
	reply(w, 200, next)
}
func (s *server) updateGiftReservation(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Evento no encontrado")
		return
	}
	var in struct {
		Version int64  `json:"version"`
		Action  string `json:"action"`
	}
	if decode(r, &in) != nil || (in.Action != "release" && in.Action != "delivered" && in.Action != "pending") {
		bad(w, 400, "Acción inválida")
		return
	}
	if in.Version != e.Wishlist.Version {
		bad(w, 409, "Hay cambios nuevos. Recarga la lista.")
		return
	}
	next := e.Wishlist
	next.Items = append([]core.Gift(nil), next.Items...)
	next.Version++
	found := false
	for i := range next.Items {
		item := &next.Items[i]
		if item.ID != r.PathValue("gift") {
			continue
		}
		found = true
		if item.GuestID == "" {
			bad(w, 409, "Este regalo aún no tiene una reserva.")
			return
		}
		if in.Action == "release" {
			item.GuestID = ""
			item.ReservedAt = nil
			item.Delivered = false
		} else {
			item.Delivered = in.Action == "delivered"
		}
	}
	if !found {
		bad(w, 404, "Regalo no encontrado")
		return
	}
	saved, err := (mongostore.Wishlists{DB: s.db}).Commit(r.Context(), e.ID, e.Wishlist.Version, next, false)
	if err != nil {
		bad(w, 500, "No pudimos actualizar la reserva.")
		return
	}
	if !saved {
		bad(w, 409, "La reserva cambió. Recarga la lista.")
		return
	}
	reply(w, 200, next)
}
func (s *server) giftMetadata(w http.ResponseWriter, r *http.Request) {
	if _, err := s.planningEvent(r); err != nil {
		bad(w, 404, "Evento no encontrado")
		return
	}
	if !s.giftRequestLimit(w, r, "gift-metadata:"+r.PathValue("id"), 30) {
		return
	}
	var in struct {
		URL string `json:"url"`
	}
	if decode(r, &in) != nil {
		bad(w, 400, "Enlace inválido")
		return
	}
	data, err := linkpreview.Fetch(r.Context(), in.URL)
	if err != nil {
		bad(w, 422, err.Error())
		return
	}
	reply(w, 200, data)
}
func (s *server) wishlistImage(w http.ResponseWriter, r *http.Request) {
	var e core.Event
	var err error
	if r.PathValue("token") != "" {
		e, _, err = s.giftInvitation(r)
	} else {
		e, err = s.planningEvent(r)
	}
	if err != nil {
		bad(w, 404, "Imagen no encontrada")
		return
	}
	key := "gift-image:" + e.ID
	if r.PathValue("token") != "" {
		key = "gift-image:" + r.PathValue("token")
	}
	if !s.giftRequestLimit(w, r, key, 240) {
		return
	}
	image := ""
	for _, item := range e.Wishlist.Items {
		if item.ID == r.PathValue("gift") {
			image = item.ImageURL
			break
		}
	}
	body, mime, err := linkpreview.Image(r.Context(), image)
	if err != nil {
		bad(w, 404, "Imagen no disponible")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Write(body)
}
