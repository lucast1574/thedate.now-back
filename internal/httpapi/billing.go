package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/checkout/session"
	"github.com/stripe/stripe-go/v86/webhook"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func stripeConfigured() bool {
	return strings.HasPrefix(os.Getenv("STRIPE_SECRET_KEY"), "sk_test_") && strings.HasPrefix(os.Getenv("STRIPE_WEBHOOK_SECRET"), "whsec_")
}

func (s *server) checkout(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.OwnerID != u.ID && u.Role != "admin" {
		bad(w, 403, "Only the event owner can pay")
		return
	}
	if e.IsDemo || e.PaymentStatus == "paid" {
		bad(w, 409, "Event already paid")
		return
	}
	if !stripeConfigured() {
		bad(w, 503, "Test payments are not configured yet")
		return
	}

	product, _ := core.ProductFor(e.Kind)
	amount, label := product.PriceCents, product.Label
	base := env("EVENT_STUDIO_URL", "https://crea.thedate.now")
	if e.Kind == "wedding" {
		base = env("WEDDING_STUDIO_URL", "https://studio.save.thedate.now")
	}
	base = strings.TrimSuffix(base, "/")
	client := session.Client{B: stripe.GetBackend(stripe.APIBackend), Key: os.Getenv("STRIPE_SECRET_KEY")}
	if e.CheckoutID != "" {
		current, getErr := client.Get(e.CheckoutID, &stripe.CheckoutSessionParams{Params: stripe.Params{Context: r.Context()}})
		if getErr != nil {
			bad(w, 502, "Could not verify existing checkout")
			return
		}
		if current.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid {
			bad(w, 409, "Payment already completed; waiting for confirmation")
			return
		}
		if current.Status == stripe.CheckoutSessionStatusOpen {
			reply(w, 200, map[string]string{"url": current.URL})
			return
		}
	}
	// One provider request per event/generation, even across processes and retries.
	var owner core.User
	if s.db.Collection("users").FindOne(r.Context(), bson.M{"_id": e.OwnerID}).Decode(&owner) != nil {
		bad(w, 404, "Event owner not found")
		return
	}
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String("payment"),
		LineItems:         []*stripe.CheckoutSessionLineItemParams{{PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{Currency: stripe.String("usd"), UnitAmount: stripe.Int64(amount), ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{Name: stripe.String(label)}}, Quantity: stripe.Int64(1)}},
		SuccessURL:        stripe.String(base + "/?payment=success&event=" + e.ID),
		CancelURL:         stripe.String(base + "/?payment=cancelled&event=" + e.ID),
		ClientReferenceID: stripe.String(e.ID),
		CustomerEmail:     stripe.String(owner.Email),
	}
	params.Context = r.Context()
	params.SetIdempotencyKey("thedate-event:" + e.ID + ":" + e.CheckoutID)
	checkout, err := client.New(params)
	if err != nil {
		bad(w, 502, "Could not start test checkout")
		return
	}
	result, err := s.db.Collection("events").UpdateOne(r.Context(), bson.M{"_id": e.ID, "paymentStatus": bson.M{"$ne": "paid"}}, bson.M{"$set": bson.M{"paymentStatus": "pending", "checkoutId": checkout.ID, "updatedAt": time.Now().UTC()}, "$addToSet": bson.M{"checkoutIds": checkout.ID}})
	if err != nil {
		bad(w, 500, "Could not store checkout")
		return
	}
	if result.MatchedCount != 1 {
		bad(w, 409, "Event already paid")
		return
	}
	reply(w, 200, map[string]string{"url": checkout.URL})
}

func (s *server) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if !stripeConfigured() {
		bad(w, 503, "Test payments are not configured yet")
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
	if event.Livemode || event.Type != "checkout.session.completed" {
		reply(w, 200, map[string]string{"status": "ignored"})
		return
	}

	client := session.Client{B: stripe.GetBackend(stripe.APIBackend), Key: os.Getenv("STRIPE_SECRET_KEY")}
	checkout, err := client.Get(event.Data.Object.ID, &stripe.CheckoutSessionParams{Params: stripe.Params{Context: r.Context()}})
	if err != nil || checkout.Livemode || checkout.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid || checkout.ClientReferenceID == "" {
		bad(w, 502, "Could not verify test payment")
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
	filter := bson.M{"_id": checkout.ClientReferenceID, "$or": bson.A{bson.M{"checkoutId": checkout.ID}, bson.M{"checkoutIds": checkout.ID}}}
	update := bson.M{"$set": bson.M{"paymentStatus": "paid", "updatedAt": time.Now().UTC()}}
	result, err := s.db.Collection("events").UpdateOne(r.Context(), filter, update)
	if err != nil {
		bad(w, 500, "Could not save payment")
		return
	}
	if result.MatchedCount != 1 {
		bad(w, 409, "Checkout was not recorded; retry")
		return
	}
	reply(w, 200, map[string]string{"status": "processed"})
}
