package mongostore

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

type Affiliates struct{ DB *mongo.Database }
type Wallet struct {
	ID          string                     `bson:"_id"`
	Code        string                     `bson:"affiliateCode" json:"code"`
	Enabled     bool                       `bson:"affiliateEnabled" json:"enabled"`
	Balance     int64                      `bson:"affiliateBalanceCents" json:"availableCents"`
	Earned      int64                      `bson:"affiliateEarnedCents" json:"earnedCents"`
	Entries     map[string]int64           `bson:"affiliateEntries" json:"-"`
	Withdrawals map[string]core.Withdrawal `bson:"withdrawals" json:"-"`
}

func (s Affiliates) Wallet(ctx context.Context, id string) (Wallet, error) {
	var w Wallet
	err := s.DB.Collection("users").FindOne(ctx, bson.M{"_id": id}).Decode(&w)
	return w, err
}
func (s Affiliates) Convert(ctx context.Context, owner string, e core.Event) error {
	if e.Payment == nil || e.PaymentSource != "stripe" {
		return nil
	}
	var user struct {
		ReferredBy string           `bson:"referredBy"`
		Conversion *core.Conversion `bson:"affiliateConversion"`
	}
	if err := s.DB.Collection("users").FindOne(ctx, bson.M{"_id": owner}).Decode(&user); err != nil {
		return err
	}
	if user.ReferredBy == "" || user.ReferredBy == owner {
		return nil
	}
	p := e.Payment
	c := core.Conversion{ReferrerID: user.ReferredBy, EventID: e.ID, SessionID: p.SessionID, PaymentCents: p.AmountCents, RefundedCents: p.RefundedCents, CommissionCents: core.AffiliateCommission(p.AmountCents, p.RefundedCents), Live: p.Live, CreatedAt: p.PaidAt}
	// Test conversions never consume the one eligible real first payment.
	field := "affiliateConversion"
	if !p.Live {
		field = "affiliateTestConversion"
	}
	_, err := s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": owner, field: bson.M{"$exists": false}}, bson.M{"$set": bson.M{field: c}})
	if err != nil {
		return err
	}
	raw := bson.M{}
	if err = s.DB.Collection("users").FindOne(ctx, bson.M{"_id": owner}).Decode(&raw); err != nil {
		return err
	}
	bytes, _ := bson.Marshal(raw[field])
	if err = bson.Unmarshal(bytes, &c); err != nil {
		return err
	}
	if c.SessionID != p.SessionID {
		return nil
	}
	if p.RefundedCents > c.RefundedCents {
		c.RefundedCents = p.RefundedCents
	}
	c.CommissionCents = core.AffiliateCommission(c.PaymentCents, c.RefundedCents)
	// CAS per affiliate document: credit marker, balance and earned total change together.
	for range 32 {
		w, err := s.Wallet(ctx, c.ReferrerID)
		if err != nil {
			return err
		}
		key := "affiliateEntries." + owner
		if !p.Live {
			key = "affiliateTestEntries." + owner
		}
		current := bson.M{}
		if err = s.DB.Collection("users").FindOne(ctx, bson.M{"_id": w.ID}).Decode(&current); err != nil {
			return err
		}
		entries := bson.M{}
		if b, ok := current[mapField(p.Live)].(bson.M); ok {
			entries = b
		} else if d, ok := current[mapField(p.Live)].(bson.D); ok {
			for _, v := range d {
				entries[v.Key] = v.Value
			}
		}
		previous, exists := entries[owner]
		old := int64(0)
		switch v := previous.(type) {
		case int64:
			old = v
		case int32:
			old = int64(v)
		}
		if exists && c.CommissionCents > old {
			c.CommissionCents = old
		}
		filter := bson.M{"_id": w.ID}
		if exists {
			filter[key] = previous
		} else {
			filter[key] = bson.M{"$exists": false}
		}
		update := bson.M{"$set": bson.M{key: c.CommissionCents}}
		if p.Live {
			update["$inc"] = bson.M{"affiliateBalanceCents": c.CommissionCents - old, "affiliateEarnedCents": c.CommissionCents - old}
		}
		result, err := s.DB.Collection("users").UpdateOne(ctx, filter, update)
		if err != nil {
			return err
		}
		if result.MatchedCount == 1 {
			_, err = s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": owner, field + ".sessionId": p.SessionID}, bson.M{"$max": bson.M{field + ".refundedCents": c.RefundedCents}, "$min": bson.M{field + ".commissionCents": c.CommissionCents}})
			return err
		}
	}
	return errors.New("affiliate conflict")
}
func mapField(live bool) string {
	if live {
		return "affiliateEntries"
	}
	return "affiliateTestEntries"
}
func (s Affiliates) Reserve(ctx context.Context, id string, w core.Withdrawal) error {
	if w.AmountCents < core.MinimumWithdrawalCents {
		return errors.New("minimum withdrawal is USD 50")
	}
	r, err := s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": id, "affiliateEnabled": true, "affiliateBalanceCents": bson.M{"$gte": w.AmountCents}, "withdrawals." + w.ID: bson.M{"$exists": false}}, bson.M{"$inc": bson.M{"affiliateBalanceCents": -w.AmountCents}, "$set": bson.M{"withdrawals." + w.ID: w}})
	if err != nil {
		return err
	}
	if r.MatchedCount != 1 {
		return errors.New("insufficient balance or duplicate request")
	}
	return nil
}
func (s Affiliates) Transition(ctx context.Context, user, id, to, note string) error {
	w, err := s.Wallet(ctx, user)
	if err != nil {
		return err
	}
	entry, ok := w.Withdrawals[id]
	if !ok || !core.WithdrawalTransition(entry.Status, to) {
		return errors.New("invalid withdrawal transition")
	}
	changes := bson.M{"$set": bson.M{"withdrawals." + id + ".status": to, "withdrawals." + id + ".note": note, "withdrawals." + id + ".updatedAt": time.Now().UTC()}}
	if to == "rejected" {
		changes["$inc"] = bson.M{"affiliateBalanceCents": entry.AmountCents}
	}
	filter := bson.M{"_id": user, "withdrawals." + id + ".status": entry.Status}
	if to != "rejected" {
		filter["affiliateBalanceCents"] = bson.M{"$gte": 0}
	}
	r, err := s.DB.Collection("users").UpdateOne(ctx, filter, changes)
	if err != nil {
		return err
	}
	if r.MatchedCount != 1 {
		return errors.New("withdrawal changed")
	}
	return nil
}
