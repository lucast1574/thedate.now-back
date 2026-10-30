package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/linkpreview"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
)

func (s *server) catalogEvent(r *http.Request) (core.Event, error) {
	var e core.Event
	if core.ValidateSlug(r.PathValue("slug")) != nil || (r.PathValue("kind") != "wedding" && r.PathValue("kind") != "general") {
		return e, core.ErrGiftDisabled
	}
	err := s.db.Collection("events").FindOne(r.Context(), bson.M{"kind": r.PathValue("kind"), "slug": r.PathValue("slug"), "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}, "isDemo": bson.M{"$ne": true}}).Decode(&e)
	return e, err
}
func (s *server) wishlistCatalog(w http.ResponseWriter, r *http.Request) {
	e, err := s.catalogEvent(r)
	if err != nil {
		bad(w, 404, "La lista no está disponible.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	reply(w, 200, publicWishlist(e, core.Guest{}))
}
func (s *server) wishlistCatalogImage(w http.ResponseWriter, r *http.Request) {
	e, err := s.catalogEvent(r)
	if err != nil {
		bad(w, 404, "Imagen no disponible")
		return
	}
	if !s.giftRequestLimit(w, r, "gift-catalog-images:"+e.ID, 240) {
		return
	}
	raw := ""
	for _, item := range e.Wishlist.Items {
		if item.ID == r.PathValue("gift") {
			raw = item.ImageURL
			break
		}
	}
	body, mime, err := linkpreview.Image(r.Context(), raw)
	if err != nil {
		bad(w, 404, "Imagen no disponible")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Write(body)
}
