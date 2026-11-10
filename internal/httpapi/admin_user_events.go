package httpapi

import (
	"net/http"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *server) adminUserEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.admin(w, r); !ok {
		return
	}
	owner := r.PathValue("user")
	if s.db.Collection("users").FindOne(r.Context(), bson.M{"_id": owner}).Err() != nil {
		bad(w, 404, "Account not found")
		return
	}
	filter := bson.M{"ownerId": owner, "deletionRequested": bson.M{"$ne": true}}
	cur, err := s.db.Collection("events").Find(r.Context(), filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		bad(w, 500, "Could not list account events")
		return
	}
	defer cur.Close(r.Context())
	events := []core.Event{}
	if cur.All(r.Context(), &events) != nil {
		bad(w, 500, "Could not list account events")
		return
	}
	reply(w, 200, events)
}
