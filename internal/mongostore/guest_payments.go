package mongostore

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

var ErrPaymentConflict = errors.New("Este comprobante o identificador ya tiene un pago registrado.")

func RecordGuestPayment(ctx context.Context, db *mongo.Database, e core.Event, p core.GuestPayment, now time.Time) error {
	if !core.ValidGuestPayment(p, e.Admission, now) || e.Kind != "general" || e.IsDemo || e.PaymentStatus != "paid" {
		return errors.New("invalid payment")
	}
	if existing, ok := e.GuestPayments[p.ID]; ok {
		if existing.GuestID == p.GuestID && existing.AmountCents == p.AmountCents && existing.MessageID == p.MessageID && existing.Currency == p.Currency && existing.PaidOn == p.PaidOn && existing.Method == p.Method && existing.Reference == p.Reference {
			return nil
		}
		return ErrPaymentConflict
	}
	if len(e.GuestPayments) >= 2000 {
		return errors.New("El evento alcanzó el límite de 2000 registros de pago.")
	}
	for _, v := range e.GuestPayments {
		if p.MessageID != "" && v.MessageID == p.MessageID {
			return ErrPaymentConflict
		}
	}
	if db.Collection("guests").FindOne(ctx, bson.M{"_id": p.GuestID, "eventId": e.ID, "sendState": bson.M{"$ne": "removing"}}).Err() != nil {
		return errors.New("guest unavailable")
	}
	if p.MessageID != "" {
		m, err := (Inbox{DB: db}).Message(ctx, p.MessageID)
		if err != nil || m.EventID != e.ID || m.GuestID != p.GuestID || m.Direction != "in" || m.Media == nil || !m.Media.Available || m.Media.Digest == "" {
			return errors.New("invalid receipt")
		}
		p.ReceiptDigest = m.Media.Digest
		for _, existing := range e.GuestPayments {
			if p.ReceiptDigest != "" && existing.ReceiptDigest == p.ReceiptDigest {
				return ErrPaymentConflict
			}
		}
	}
	filter := bson.M{"_id": e.ID, "responseVersion": e.ResponseVersion, "guestPayments." + p.ID: bson.M{"$exists": false}, "deletionRequested": bson.M{"$ne": true}}
	result, err := db.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"guestPayments." + p.ID: p}, "$inc": bson.M{"responseVersion": 1}})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return ErrPaymentConflict
	}
	return nil
}
