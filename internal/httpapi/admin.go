package httpapi

import (
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"net/http"
	"time"
)

func (s *server) admin(w http.ResponseWriter, r *http.Request) (core.User, bool) {
	u, e := s.user(r)
	if e != nil {
		bad(w, 401, "Sign in required")
		return u, false
	}
	if u.Role != "admin" {
		bad(w, 403, "Administrator required")
		return u, false
	}
	return u, true
}
func (s *server) audit(r *http.Request, u core.User, action, target, reason string) error {
	_, err := s.db.Collection("adminAudit").InsertOne(r.Context(), bson.M{"_id": uuid.NewString(), "actorId": u.ID, "action": action, "targetId": target, "reason": reason, "createdAt": time.Now().UTC()})
	return err
}
func (s *server) adminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.admin(w, r); !ok {
		return
	}
	filter := bson.M{}
	email := r.URL.Query().Get("email")
	if email != "" {
		filter["email"] = email
	}
	cur, err := s.db.Collection("users").Find(r.Context(), filter, options.Find().SetLimit(100).SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		bad(w, 500, "Could not list accounts")
		return
	}
	defer cur.Close(r.Context())
	users := []core.User{}
	if cur.All(r.Context(), &users) != nil {
		bad(w, 500, "Could not list accounts")
		return
	}
	type account struct {
		core.User
		CanBeAdmin    bool `json:"canBeAdmin"`
		RoleProtected bool `json:"roleProtected"`
	}
	accounts := make([]account, 0, len(users))
	for _, user := range users {
		accounts = append(accounts, account{User: user, CanBeAdmin: user.GoogleSub != "" && user.IdentityPolicy == 1 && user.GoogleAuthoritative, RoleProtected: user.Email == adminEmail()})
	}
	reply(w, 200, accounts)
}
func (s *server) adminRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.admin(w, r)
	if !ok {
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if decode(r, &in) != nil || !core.ValidRole(in.Role) {
		bad(w, 400, "Invalid role")
		return
	}
	var target core.User
	if s.db.Collection("users").FindOne(r.Context(), bson.M{"_id": r.PathValue("user")}).Decode(&target) != nil {
		bad(w, 404, "Account not found")
		return
	}
	if target.Email == adminEmail() || target.ID == actor.ID {
		bad(w, 403, "The owner and your own role are protected")
		return
	}
	if in.Role == "admin" && (target.GoogleSub == "" || target.IdentityPolicy != 1 || !target.GoogleAuthoritative) {
		bad(w, 403, "Administrators must first sign in with an authoritative Google identity")
		return
	}
	if s.audit(r, actor, "role-request", target.ID, in.Role) != nil {
		bad(w, 500, "Could not audit role change")
		return
	}
	changed, err := mongostore.ChangeUserRole(r.Context(), s.db, target, in.Role)
	if err != nil || !changed {
		bad(w, 409, "Account changed; reload")
		return
	}
	if s.audit(r, actor, "role-changed", target.ID, in.Role) != nil {
		bad(w, 500, "Role changed; audit acknowledgement unavailable")
		return
	}
	reply(w, 200, map[string]string{"status": "updated"})
}
func (s *server) adminCourtesy(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.admin(w, r)
	if !ok {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if decode(r, &in) != nil || len(in.Reason) < 3 || len(in.Reason) > 300 {
		bad(w, 400, "Provide a courtesy reason")
		return
	}
	id := r.PathValue("id")
	if s.audit(r, actor, "courtesy-request", id, in.Reason) != nil {
		bad(w, 500, "Could not audit courtesy")
		return
	}
	result, err := s.db.Collection("events").UpdateOne(r.Context(), bson.M{"_id": id, "isDemo": bson.M{"$ne": true}, "paymentStatus": bson.M{"$ne": "paid"}, "checkoutId": bson.M{"$in": bson.A{nil, ""}}}, bson.M{"$set": bson.M{"paymentStatus": "paid", "paymentSource": "courtesy", "courtesy": bson.M{"actorId": actor.ID, "reason": in.Reason, "at": time.Now().UTC()}, "updatedAt": time.Now().UTC()}})
	if err != nil || result.MatchedCount != 1 {
		bad(w, 409, "Only unpaid events without active checkout can receive a courtesy")
		return
	}
	if s.audit(r, actor, "courtesy-granted", id, in.Reason) != nil {
		bad(w, 500, "Courtesy granted; audit acknowledgement unavailable")
		return
	}
	reply(w, 200, map[string]string{"status": "granted"})
}
