package httpapi

import (
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"net/http"
)

func (s *server) adminWithdrawals(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.admin(w, r); !ok {
		return
	}
	cur, err := s.db.Collection("users").Find(r.Context(), bson.M{"withdrawals": bson.M{"$exists": true}}, options.Find().SetLimit(100))
	if err != nil {
		bad(w, 500, "Could not load withdrawals")
		return
	}
	defer cur.Close(r.Context())
	rows := []map[string]any{}
	for cur.Next(r.Context()) {
		var user struct {
			ID          string                     `bson:"_id"`
			Name        string                     `bson:"name"`
			Email       string                     `bson:"email"`
			Balance     int64                      `bson:"affiliateBalanceCents"`
			Withdrawals map[string]core.Withdrawal `bson:"withdrawals"`
		}
		if cur.Decode(&user) != nil {
			bad(w, 500, "Could not load withdrawals")
			return
		}
		for _, entry := range user.Withdrawals {
			rows = append(rows, map[string]any{"userId": user.ID, "name": user.Name, "email": user.Email, "balanceCents": user.Balance, "withdrawal": entry})
		}
	}
	if cur.Err() != nil {
		bad(w, 500, "Could not load withdrawals")
		return
	}
	reply(w, 200, rows)
}
func (s *server) adminWithdrawal(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.admin(w, r)
	if !ok {
		return
	}
	var in struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if decode(r, &in) != nil || len(in.Note) < 3 || len(in.Note) > 500 {
		bad(w, 400, "A reason or payment reference is required")
		return
	}
	user, id := r.PathValue("user"), r.PathValue("withdrawal")
	if _, err := uuid.Parse(user); err != nil {
		bad(w, 400, "Invalid account")
		return
	}
	if _, err := uuid.Parse(id); err != nil {
		bad(w, 400, "Invalid withdrawal")
		return
	}
	if s.audit(r, actor, "withdrawal-request", user+"/"+id, in.Status+": "+in.Note) != nil {
		bad(w, 500, "Could not audit withdrawal")
		return
	}
	if (mongostore.Affiliates{DB: s.db}).Transition(r.Context(), user, id, in.Status, in.Note) != nil {
		bad(w, 409, "Invalid or concurrent withdrawal transition")
		return
	}
	if s.audit(r, actor, "withdrawal-updated", user+"/"+id, in.Status) != nil {
		bad(w, 500, "Withdrawal updated; audit acknowledgement unavailable")
		return
	}
	reply(w, 200, map[string]string{"status": "updated"})
}
