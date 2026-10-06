package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"time"
)

func (s *server) listCollaborators(w http.ResponseWriter, r *http.Request) {
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	e, err = (mongostore.Couples{DB: s.db}).Snapshot(r.Context(), e.ID)
	if err != nil {
		bad(w, 500, "Could not load access")
		return
	}
	pending := []core.CoupleInvite{}
	for _, i := range core.ActiveCoupleInvites(e, time.Now().UTC()) {
		pending = append(pending, i)
	}
	members := []core.User{}
	cur, err := s.db.Collection("users").Find(r.Context(), bson.M{"_id": bson.M{"$in": e.CoupleUserIDs}})
	if err != nil {
		bad(w, 500, "Could not list members")
		return
	}
	defer cur.Close(r.Context())
	if cur.All(r.Context(), &members) != nil {
		bad(w, 500, "Could not list members")
		return
	}
	reply(w, 200, map[string]any{"members": members, "pending": pending, "limit": 2, "emailConfigured": mailConfigured()})
}
func (s *server) revokeCollaborator(w http.ResponseWriter, r *http.Request) {
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	repo := mongostore.Couples{DB: s.db}
	id := r.PathValue("member")
	for range 32 {
		e, err = repo.Snapshot(r.Context(), e.ID)
		if err != nil {
			break
		}
		ids := []string{}
		for _, u := range e.CoupleUserIDs {
			if u != id {
				ids = append(ids, u)
			}
		}
		invites := core.ActiveCoupleInvites(e, time.Now().UTC())
		for hash, i := range invites {
			if i.ID == id {
				delete(invites, hash)
			}
		}
		ok, saveErr := repo.Save(r.Context(), e, ids, invites)
		if saveErr != nil {
			err = saveErr
			break
		}
		if ok {
			reply(w, 200, map[string]string{"status": "removed"})
			return
		}
	}
	bad(w, 409, "Access changed; reload before retrying")
}
