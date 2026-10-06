package httpapi

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/checkout/session"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func stripeConfigured() bool {
	return (strings.HasPrefix(os.Getenv("STRIPE_SECRET_KEY"), "sk_test_") || stripeLive()) && strings.HasPrefix(os.Getenv("STRIPE_WEBHOOK_SECRET"), "whsec_")
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
		bad(w, 503, "Payments are not configured yet")
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
		bad(w, 502, "Could not start checkout")
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
