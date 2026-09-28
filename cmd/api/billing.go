package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/checkout/session"
	"github.com/stripe/stripe-go/v86/webhook"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func stripeConfigured() bool {
	return strings.HasPrefix(os.Getenv("STRIPE_SECRET_KEY"), "sk_test_") && strings.HasPrefix(os.Getenv("STRIPE_WEDDING_PRICE_ID"), "price_") && strings.HasPrefix(os.Getenv("STRIPE_EVENT_PRICE_ID"), "price_") && strings.HasPrefix(os.Getenv("STRIPE_WEBHOOK_SECRET"), "whsec_")
}

func (s *server) checkout(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.OwnerID != u.ID {
		bad(w, 403, "Only the event owner can pay")
		return
	}
	if e.PaymentStatus == "paid" {
		bad(w, 409, "Event already paid")
		return
	}
	if !stripeConfigured() {
		bad(w, 503, "Test payments are not configured yet")
		return
	}
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	price := os.Getenv("STRIPE_EVENT_PRICE_ID")
	if e.Kind == "wedding" {
		price = os.Getenv("STRIPE_WEDDING_PRICE_ID")
	}
	base := strings.TrimSuffix(env("BACKOFFICE_URL", "https://backoffice.thedate.now"), "/")
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String("payment"),
		LineItems:         []*stripe.CheckoutSessionLineItemParams{{Price: stripe.String(price), Quantity: stripe.Int64(1)}},
		SuccessURL:        stripe.String(base + "/?payment=success&event=" + e.ID),
		CancelURL:         stripe.String(base + "/?payment=cancelled&event=" + e.ID),
		ClientReferenceID: stripe.String(e.ID),
		CustomerEmail:     stripe.String(u.Email),
	}
	checkout, err := session.New(params)
	if err != nil {
		bad(w, 502, "Could not start test checkout")
		return
	}
	_, err = s.db.Collection("events").UpdateOne(r.Context(), bson.M{"_id": e.ID, "paymentStatus": bson.M{"$ne": "paid"}}, bson.M{"$set": bson.M{"paymentStatus": "pending", "checkoutId": checkout.ID, "updatedAt": time.Now().UTC()}})
	if err != nil {
		bad(w, 500, "Could not store checkout")
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
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	checkout, err := session.Get(event.Data.Object.ID, nil)
	if err != nil || checkout.Livemode || checkout.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid || checkout.ClientReferenceID == "" {
		bad(w, 502, "Could not verify test payment")
		return
	}
	filter := bson.M{"_id": checkout.ClientReferenceID, "checkoutId": checkout.ID, "paymentStatus": "pending"}
	update := bson.M{"$set": bson.M{"paymentStatus": "paid", "updatedAt": time.Now().UTC()}}
	_, err = s.db.Collection("events").UpdateOne(r.Context(), filter, update)
	if err != nil {
		bad(w, 500, "Could not save payment")
		return
	}
	reply(w, 200, map[string]string{"status": "processed"})
}

func (s *server) publish(w http.ResponseWriter, r *http.Request) {
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.PaymentStatus != "paid" {
		bad(w, 403, "Complete test checkout before publishing")
		return
	}
	now := time.Now().UTC()
	_, err = s.db.Collection("events").UpdateByID(r.Context(), e.ID, bson.M{"$set": bson.M{"publishedAt": now, "updatedAt": now}})
	if err != nil {
		bad(w, 500, "Could not publish invitation")
		return
	}
	reply(w, 200, map[string]any{"publishedAt": now})
}
