package httpapi

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/charge"
	"github.com/stripe/stripe-go/v86/dispute"
	"github.com/stripe/stripe-go/v86/paymentintent"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"os"
	"strings"
	"time"
)

func stripeLive() bool { return strings.HasPrefix(os.Getenv("STRIPE_SECRET_KEY"), "sk_live_") }
func (s *server) recordPayment(ctx context.Context, e core.Event, checkout *stripe.CheckoutSession) error {
	intent := ""
	if checkout.PaymentIntent != nil {
		intent = checkout.PaymentIntent.ID
	}
	if intent == "" {
		return errors.New("missing payment intent")
	}
	params := &stripe.PaymentIntentParams{Params: stripe.Params{Context: ctx}}
	params.AddExpand("latest_charge")
	verified, err := (paymentintent.Client{B: stripe.GetBackend(stripe.APIBackend), Key: os.Getenv("STRIPE_SECRET_KEY")}).Get(intent, params)
	if err != nil || verified.Livemode != checkout.Livemode || verified.AmountReceived != checkout.AmountTotal || verified.Currency != stripe.CurrencyUSD || verified.LatestCharge == nil {
		return errors.New("unverified payment intent")
	}
	refunded := verified.LatestCharge.AmountRefunded
	if verified.LatestCharge.Disputed {
		refunded = checkout.AmountTotal
	}
	p := core.Payment{SessionID: checkout.ID, IntentID: intent, AmountCents: checkout.AmountTotal, RefundedCents: refunded, Currency: "USD", Live: checkout.Livemode, PaidAt: time.Unix(checkout.Created, 0).UTC()}
	filter := bson.M{"_id": e.ID, "paymentSource": bson.M{"$ne": "courtesy"}, "$or": bson.A{bson.M{"checkoutId": checkout.ID}, bson.M{"checkoutIds": checkout.ID}}, "payment": bson.M{"$exists": false}}
	_, err = s.db.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"payment": p, "paymentSource": "stripe", "paymentStatus": "paid", "updatedAt": time.Now().UTC()}})
	if err != nil {
		return err
	}
	// Retries read the canonical stored payment and never reset refund totals.
	if err = s.db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID, "payment.sessionId": checkout.ID, "paymentSource": "stripe", "$or": bson.A{bson.M{"checkoutId": checkout.ID}, bson.M{"checkoutIds": checkout.ID}}}).Decode(&e); err != nil {
		return err
	}
	return (mongostore.Affiliates{DB: s.db}).Convert(ctx, e.OwnerID, e)
}
func (s *server) paymentReversal(w http.ResponseWriter, r *http.Request, kind, id string) {
	client := charge.Client{B: stripe.GetBackend(stripe.APIBackend), Key: os.Getenv("STRIPE_SECRET_KEY")}
	chargeID := id
	disputed := false
	if strings.HasPrefix(kind, "charge.dispute.") {
		d, err := (dispute.Client{B: stripe.GetBackend(stripe.APIBackend), Key: os.Getenv("STRIPE_SECRET_KEY")}).Get(id, &stripe.DisputeParams{Params: stripe.Params{Context: r.Context()}})
		if err != nil || d.Livemode != stripeLive() || d.Charge == nil {
			bad(w, 502, "Could not verify dispute")
			return
		}
		chargeID = d.Charge.ID
		disputed = d.Status != stripe.DisputeStatusWon
	}
	c, err := client.Get(chargeID, &stripe.ChargeParams{Params: stripe.Params{Context: r.Context()}})
	if err != nil || c.Livemode != stripeLive() || c.PaymentIntent == nil || c.Currency != stripe.CurrencyUSD {
		bad(w, 502, "Could not verify refund")
		return
	}
	var e core.Event
	if s.db.Collection("events").FindOne(r.Context(), bson.M{"payment.intentId": c.PaymentIntent.ID, "paymentSource": "stripe"}).Decode(&e) != nil {
		reply(w, 200, map[string]string{"status": "ignored"})
		return
	}
	refunded := c.AmountRefunded
	if disputed {
		refunded = e.Payment.AmountCents
	}
	if refunded < 0 || refunded > e.Payment.AmountCents {
		bad(w, 400, "Unexpected refund amount")
		return
	}
	// A disputed commission remains withheld even after a later won dispute;
	// reinstatement requires an audited review instead of replaying old events.
	_, err = s.db.Collection("events").UpdateByID(r.Context(), e.ID, bson.M{"$max": bson.M{"payment.refundedCents": refunded}, "$set": bson.M{"updatedAt": time.Now().UTC()}})
	if err != nil {
		bad(w, 500, "Could not save refund")
		return
	}
	if s.db.Collection("events").FindOne(r.Context(), bson.M{"_id": e.ID}).Decode(&e) != nil || (mongostore.Affiliates{DB: s.db}).Convert(r.Context(), e.OwnerID, e) != nil {
		bad(w, 500, "Could not reconcile refund")
		return
	}
	reply(w, 200, map[string]string{"status": "processed"})
}
