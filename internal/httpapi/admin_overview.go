package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"net/http"
)

type productRevenue struct {
	Events     int   `json:"events"`
	Courtesies int   `json:"courtesies"`
	Gross      int64 `json:"grossCents"`
	Refunded   int64 `json:"refundedCents"`
	Net        int64 `json:"netCents"`
	Test       int64 `json:"testCents"`
}

func (s *server) adminOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.admin(w, r); !ok {
		return
	}
	cur, err := s.db.Collection("events").Find(r.Context(), bson.M{"isDemo": bson.M{"$ne": true}}, options.Find().SetProjection(bson.M{"kind": 1, "payment": 1, "paymentSource": 1}))
	if err != nil {
		bad(w, 500, "Could not load income")
		return
	}
	defer cur.Close(r.Context())
	totals := map[string]productRevenue{"wedding": {}, "general": {}}
	for cur.Next(r.Context()) {
		var e core.Event
		if cur.Decode(&e) != nil {
			bad(w, 500, "Could not load income")
			return
		}
		v := totals[e.Kind]
		v.Events++
		if e.PaymentSource == "courtesy" {
			v.Courtesies++
		}
		if p := e.Payment; p != nil && e.PaymentSource == "stripe" {
			if p.Live {
				v.Gross += p.AmountCents
				v.Refunded += p.RefundedCents
				v.Net += p.AmountCents - p.RefundedCents
			} else {
				v.Test += p.AmountCents - p.RefundedCents
			}
		}
		totals[e.Kind] = v
	}
	if cur.Err() != nil {
		bad(w, 500, "Could not load income")
		return
	}
	count, err := s.db.Collection("users").CountDocuments(r.Context(), bson.M{})
	if err != nil {
		bad(w, 500, "Could not count users")
		return
	}
	reply(w, 200, map[string]any{"products": totals, "users": count, "currency": "USD", "testMode": !stripeLive(), "commissionBps": core.CommissionBPS, "minimumWithdrawalCents": core.MinimumWithdrawalCents})
}
