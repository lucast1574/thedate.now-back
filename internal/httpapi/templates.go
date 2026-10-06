package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
)

func (s *server) listTemplates(w http.ResponseWriter, r *http.Request) {
	if _, err := s.user(r); err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	cur, err := s.db.Collection("invitationTemplates").Find(r.Context(), bson.M{})
	if err != nil {
		bad(w, 500, "Could not load templates")
		return
	}
	defer cur.Close(r.Context())
	list := []core.InvitationTemplate{}
	if err = cur.All(r.Context(), &list); err != nil {
		bad(w, 500, "Could not load templates")
		return
	}
	reply(w, 200, list)
}
