package httpapi

import (
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

var affiliatePattern = regexp.MustCompile(`^[a-f0-9]{24}$`)

func (s *server) referredBy(r *http.Request, code string) string {
	if !affiliatePattern.MatchString(code) {
		return ""
	}
	var u core.User
	if s.db.Collection("users").FindOne(r.Context(), bson.M{"affiliateCode": code, "affiliateEnabled": true, "role": bson.M{"$ne": "admin"}}).Decode(&u) != nil {
		return ""
	}
	return u.ID
}
func (s *server) affiliateJoin(w http.ResponseWriter, r *http.Request) {
	u, e := s.user(r)
	if e != nil {
		bad(w, 401, "Sign in required")
		return
	}
	if u.Role == "admin" {
		bad(w, 403, "Admin courtesy accounts are not commission eligible")
		return
	}
	code := strings.ReplaceAll(uuid.NewString(), "-", "")[:24]
	_, e = s.db.Collection("users").UpdateOne(r.Context(), bson.M{"_id": u.ID, "affiliateEnabled": bson.M{"$ne": true}}, bson.M{"$set": bson.M{"affiliateCode": code, "affiliateEnabled": true}})
	if e != nil {
		bad(w, 500, "Could not enable affiliates")
		return
	}
	s.affiliateOverview(w, r)
}
func (s *server) affiliateOverview(w http.ResponseWriter, r *http.Request) {
	u, e := s.user(r)
	if e != nil {
		bad(w, 401, "Sign in required")
		return
	}
	wallet, e := (mongostore.Affiliates{DB: s.db}).Wallet(r.Context(), u.ID)
	if e != nil {
		bad(w, 500, "Could not load affiliate account")
		return
	}
	rows := []core.Withdrawal{}
	for _, v := range wallet.Withdrawals {
		rows = append(rows, v)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	count, e := s.db.Collection("users").CountDocuments(r.Context(), bson.M{"referredBy": u.ID})
	if e != nil {
		bad(w, 500, "Could not load referrals")
		return
	}
	reply(w, 200, map[string]any{"code": wallet.Code, "enabled": wallet.Enabled, "availableCents": wallet.Balance, "earnedCents": wallet.Earned, "referrals": count, "conversions": len(wallet.Entries), "withdrawals": rows, "commissionBps": core.CommissionBPS, "minimumWithdrawalCents": core.MinimumWithdrawalCents})
}
func (s *server) affiliateWithdraw(w http.ResponseWriter, r *http.Request) {
	u, e := s.user(r)
	if e != nil {
		bad(w, 401, "Sign in required")
		return
	}
	var in struct {
		Amount  int64  `json:"amountCents"`
		Method  string `json:"method"`
		Details string `json:"details"`
	}
	if decode(r, &in) != nil || in.Amount < core.MinimumWithdrawalCents || in.Amount > 100000000 || len(in.Method) < 2 || len(in.Method) > 40 || len(in.Details) < 5 || len(in.Details) > 500 || u.Role == "admin" {
		bad(w, 400, "Valid payment details and minimum USD 50 required")
		return
	}
	now := time.Now().UTC()
	entry := core.Withdrawal{ID: uuid.NewString(), AmountCents: in.Amount, Method: in.Method, Details: in.Details, Status: "pending", CreatedAt: now, UpdatedAt: now}
	if e = (mongostore.Affiliates{DB: s.db}).Reserve(r.Context(), u.ID, entry); e != nil {
		bad(w, 409, "Insufficient available balance")
		return
	}
	reply(w, 201, entry)
}
