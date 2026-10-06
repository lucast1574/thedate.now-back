package httpapi

import (
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/checkout/session"
	"github.com/stripe/stripe-go/v86/webhook"
	"go.mongodb.org/mongo-driver/v2/bson"
	"io"
	"net/http"
	"os"
)

func (s *server) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if !stripeConfigured() {
		bad(w, 503, "Payments are not configured yet")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		bad(w, 400, "Invalid body")
		return
	}
	if err = webhook.ValidatePayload(body, r.Header.Get("Stripe-Signature"), os.Getenv("STRIPE_WEBHOOK_SECRET")); err != nil {
		bad(w, 400, "Invalid signature")
		return
	}
	var event struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Livemode bool   `json:"livemode"`
		Data     struct {
			Object struct {
				ID string `json:"id"`
			} `json:"object"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &event) != nil || event.ID == "" {
		bad(w, 400, "Invalid event")
		return
	}
	if event.Livemode != stripeLive() {
		reply(w, 200, map[string]string{"status": "ignored"})
		return
	}
	if event.Type == "charge.refunded" || event.Type == "charge.dispute.created" || event.Type == "charge.dispute.closed" {
		s.paymentReversal(w, r, event.Type, event.Data.Object.ID)
		return
	}
	if event.Type != "checkout.session.completed" && event.Type != "checkout.session.async_payment_succeeded" {
		reply(w, 200, map[string]string{"status": "ignored"})
		return
	}

	client := session.Client{B: stripe.GetBackend(stripe.APIBackend), Key: os.Getenv("STRIPE_SECRET_KEY")}
	checkout, err := client.Get(event.Data.Object.ID, &stripe.CheckoutSessionParams{Params: stripe.Params{Context: r.Context()}})
	if err != nil || checkout.Livemode != stripeLive() || checkout.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid || checkout.ClientReferenceID == "" {
		bad(w, 502, "Could not verify payment")
		return
	}
	var paidEvent core.Event
	if s.db.Collection("events").FindOne(r.Context(), bson.M{"_id": checkout.ClientReferenceID}).Decode(&paidEvent) != nil {
		bad(w, 409, "Unknown payment event")
		return
	}
	product, productErr := core.ProductFor(paidEvent.Kind)
	if productErr != nil || checkout.AmountTotal != product.PriceCents || checkout.Currency != stripe.CurrencyUSD || checkout.Mode != stripe.CheckoutSessionModePayment {
		bad(w, 400, "Unexpected payment amount or currency")
		return
	}
	if err = s.recordPayment(r.Context(), paidEvent, checkout); err != nil {
		bad(w, 409, "Could not reconcile payment; retry")
		return
	}
	reply(w, 200, map[string]string{"status": "processed"})
}
