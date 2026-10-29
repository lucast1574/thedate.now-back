package mongostore

import (
	"context"
	"errors"
	"fmt"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

func (s Affiliates) ConvertPartner(ctx context.Context, e core.Event, q core.CheckoutQuote) error {
	if e.Payment == nil || !e.Payment.Live {
		return nil
	}
	amount := core.PartnerCommission(q, *e.Payment)
	key := "partnerEntries." + e.ID
	for range 32 {
		var account struct {
			Entries map[string]core.PartnerEntry `bson:"partnerEntries"`
		}
		if err := s.DB.Collection("users").FindOne(ctx, bson.M{"_id": q.PartnerID, "role": bson.M{"$ne": "admin"}}).Decode(&account); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil
			}
			return err
		}
		old, exists := account.Entries[e.ID]
		entry := core.PartnerEntry{EventID: e.ID, Kind: e.Kind, Title: e.Title, Amount: amount, AvailableAt: e.Payment.PaidAt.Add(core.PartnerHold)}
		filter := bson.M{"_id": q.PartnerID}
		if exists {
			filter[key] = old
			entry = old
			if amount < entry.Amount {
				entry.Amount = amount
			}
		} else {
			filter[key] = bson.M{"$exists": false}
		}
		credited := entry.Credited
		// Refunds revoke previously released credit, even after it was spent.
		if credited > entry.Amount {
			credited = entry.Amount
		}
		delta := credited - entry.Credited
		entry.Credited = credited
		update := bson.M{"$set": bson.M{key: entry}}
		if delta != 0 {
			update["$inc"] = bson.M{"affiliateBalanceCents": delta, "affiliateEarnedCents": delta}
		}
		r, err := s.DB.Collection("users").UpdateOne(ctx, filter, update)
		if err != nil {
			return err
		}
		if r.MatchedCount == 1 {
			return nil
		}
	}
	return fmt.Errorf("partner credit conflict")
}
func (s Affiliates) PartnerEntries(ctx context.Context, id string, now time.Time) ([]core.PartnerEntry, error) {
	var account struct {
		Entries map[string]core.PartnerEntry `bson:"partnerEntries"`
	}
	if err := s.DB.Collection("users").FindOne(ctx, bson.M{"_id": id}).Decode(&account); err != nil {
		return nil, err
	}
	entries := []core.PartnerEntry{}
	for key, entry := range account.Entries {
		if !now.Before(entry.AvailableAt) && entry.Amount > entry.Credited {
			next := entry
			next.Credited = entry.Amount
			delta := next.Credited - entry.Credited
			r, err := s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": id, "partnerEntries." + key: entry}, bson.M{"$set": bson.M{"partnerEntries." + key: next}, "$inc": bson.M{"affiliateBalanceCents": delta, "affiliateEarnedCents": delta}})
			if err != nil {
				return nil, err
			}
			if r.MatchedCount == 1 {
				entry = next
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
