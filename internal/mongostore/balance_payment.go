package mongostore

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

var errBalanceConflict = errors.New("el pago cambió; recarga o reintenta para reconciliarlo sin otro cargo")

func (s Affiliates) PayBalance(ctx context.Context, e core.Event, u core.User, amount int64, now time.Time) error {
	if e.CheckoutID != "" {
		return errors.New("ya hay un pago de Stripe iniciado; usa el saldo en un evento nuevo")
	}
	if u.Role == "admin" || amount <= 0 {
		return errors.New("este pago no admite saldo")
	}
	if e.PaymentStatus != "balance_pending" {
		wallet, err := s.Wallet(ctx, u.ID)
		if err != nil {
			return err
		}
		if wallet.Balance < amount {
			return errors.New("no tienes saldo suficiente para pagar el evento completo")
		}
		id := uuid.NewString()
		r, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": e.ID, "ownerId": u.ID, "paymentStatus": bson.M{"$nin": bson.A{"paid", "balance_pending"}}, "checkoutId": bson.M{"$in": bson.A{nil, ""}}}, bson.M{"$set": bson.M{"paymentStatus": "balance_pending", "balanceReservationId": id, "balanceAmount": amount}})
		if err != nil {
			return err
		}
		if r.MatchedCount == 0 {
			return errBalanceConflict
		}
		e.BalanceReservationID = id
		e.BalanceAmount = amount
	}
	if e.BalanceReservationID == "" || e.BalanceAmount <= 0 {
		return errBalanceConflict
	}
	key := "eventRedemptions." + e.BalanceReservationID
	amount = e.BalanceAmount
	var wallet struct {
		Redemptions map[string]int64 `bson:"eventRedemptions"`
	}
	if err := s.DB.Collection("users").FindOne(ctx, bson.M{"_id": u.ID}).Decode(&wallet); err != nil {
		return err
	}
	reserved, exists := wallet.Redemptions[e.BalanceReservationID]
	if !exists {
		_, err := s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": u.ID, key: bson.M{"$exists": false}, "affiliateBalanceCents": bson.M{"$gte": amount}}, bson.M{"$set": bson.M{key: amount}, "$inc": bson.M{"affiliateBalanceCents": -amount}})
		if err != nil {
			return err
		}
		if err = s.DB.Collection("users").FindOne(ctx, bson.M{"_id": u.ID}).Decode(&wallet); err != nil {
			return err
		}
		reserved = wallet.Redemptions[e.BalanceReservationID]
	}
	if reserved != amount {
		return errors.New("no se reservó saldo; reintenta o cancela este intento para pagar con Stripe")
	}
	filter := bson.M{"_id": e.ID, "ownerId": u.ID, "paymentStatus": "balance_pending", "balanceReservationId": e.BalanceReservationID}
	r, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"paymentStatus": "paid", "paymentSource": "balance", "updatedAt": now}})
	if mongo.IsDuplicateKeyError(err) && e.PublishedAt == nil {
		r, err = s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"paymentStatus": "paid", "paymentSource": "balance", "slug": PaidAddressFallback(e), "addressChangedAfterPayment": true, "updatedAt": now}})
	}
	if err != nil {
		return err
	}
	if r.MatchedCount == 0 {
		if s.DB.Collection("events").FindOne(ctx, bson.M{"_id": e.ID, "paymentStatus": "paid", "paymentSource": "balance", "balanceReservationId": e.BalanceReservationID}).Err() != nil {
			return errBalanceConflict
		}
	}
	return nil
}
func (s Affiliates) CancelBalance(ctx context.Context, e core.Event, u core.User) error {
	if e.PaymentStatus != "balance_pending" || e.BalanceReservationID == "" {
		return errBalanceConflict
	}
	key := "eventRedemptions." + e.BalanceReservationID
	// A zero tombstone fences every stale debit before unlocking the event.
	_, err := s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": u.ID, key: bson.M{"$exists": false}}, bson.M{"$set": bson.M{key: int64(0)}})
	if err != nil {
		return err
	}
	if s.DB.Collection("users").FindOne(ctx, bson.M{"_id": u.ID, key: int64(0)}).Err() != nil {
		return errors.New("el saldo ya está reservado; reintenta el pago con saldo para completar la activación")
	}
	_, err = s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": e.ID, "ownerId": u.ID, "paymentStatus": "balance_pending", "balanceReservationId": e.BalanceReservationID}, bson.M{"$set": bson.M{"paymentStatus": "unpaid"}})
	return err
}
