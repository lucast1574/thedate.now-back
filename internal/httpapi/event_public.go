package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
)

func (s *server) publicEvent(w http.ResponseWriter, r *http.Request) {
	kind, slug := r.PathValue("kind"), r.PathValue("slug")
	if (kind != "wedding" && kind != "general") || core.ValidateSlug(slug) != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	var e core.Event
	err := s.db.Collection("events").FindOne(r.Context(), bson.M{"kind": kind, "slug": slug, "paymentStatus": "paid", "addressReleasedAt": nil, "publishedAt": bson.M{"$ne": nil}}).Decode(&e)
	if err != nil || e.DeletionRequested {
		bad(w, 404, "Invitation not found")
		return
	}
	e = core.PublishedEvent(e)
	s.completeLegacyEvent(r.Context(), &e)
	reply(w, 200, publicEventView(e))
}
