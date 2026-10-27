package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
)

func (s *server) becomeProfessional(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	kind := s.product
	if kind == "" {
		kind = u.Product
	}
	p, err := core.ProductFor(kind)
	if err != nil {
		bad(w, 400, "Product required")
		return
	}
	if u.Role == "admin" || u.Role == p.Role {
		reply(w, 200, u)
		return
	}
	if u.Role != "user" {
		bad(w, 403, "User account required")
		return
	}
	result, err := s.db.Collection("users").UpdateOne(r.Context(), bson.M{"_id": u.ID, "role": "user"}, bson.M{"$set": bson.M{"role": p.Role}})
	if err != nil || result.MatchedCount != 1 {
		bad(w, 409, "Account changed; reload")
		return
	}
	u.Role = p.Role
	reply(w, 200, u)
}
