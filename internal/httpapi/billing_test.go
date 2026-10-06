package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSignedPaymentWebhookAcceptsEarlierIssuedCheckout(t *testing.T) {
	db := testutil.Mongo(t)
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_fake")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_fake")
	var amount atomic.Int64
	amount.Store(500)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/payment_intents/pi_first" {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "pi_first", "object": "payment_intent", "livemode": false, "amount_received": amount.Load(), "currency": "usd", "latest_charge": map[string]any{"id": "ch_first", "object": "charge", "amount_refunded": 0}})
			return
		}
		if r.URL.Path != "/v1/checkout/sessions/first" {
			t.Errorf("unexpected Stripe path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "first", "object": "checkout.session", "client_reference_id": "event", "payment_status": "paid", "payment_intent": "pi_first", "livemode": false, "mode": "payment", "currency": "usd", "amount_total": amount.Load()})
	}))
	defer provider.Close()
	previous := stripe.GetBackend(stripe.APIBackend)
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{URL: stripe.String(provider.URL), HTTPClient: provider.Client(), MaxNetworkRetries: stripe.Int64(0)}))
	defer stripe.SetBackend(stripe.APIBackend, previous)
	_, err := db.Collection("events").InsertOne(context.Background(), bson.M{"_id": "event", "ownerId": "owner", "kind": "general", "paymentStatus": "pending", "checkoutId": "second", "checkoutIds": bson.A{"first", "second"}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Collection("users").InsertOne(context.Background(), core.User{ID: "owner"})
	s := &server{db: db}
	body := `{"id":"evt_1","type":"checkout.session.completed","livemode":false,"data":{"object":{"id":"first"}}}`
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: []byte(body), Secret: "whsec_fake"})
	deliver := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/webhooks/stripe", strings.NewReader(body))
		r.Header.Set("Stripe-Signature", signed.Header)
		w := httptest.NewRecorder()
		s.stripeWebhook(w, r)
		return w
	}
	amount.Store(1)
	if w := deliver(); w.Code != 400 {
		t.Fatalf("wrong amount accepted: %d", w.Code)
	}
	amount.Store(500)
	for i := 0; i < 2; i++ {
		if w := deliver(); w.Code != 200 {
			t.Fatalf("valid/replayed payment: %d %s", w.Code, w.Body.String())
		}
	}
	var e core.Event
	if err = db.Collection("events").FindOne(context.Background(), bson.M{"_id": "event"}).Decode(&e); err != nil {
		t.Fatal(err)
	}
	if e.PaymentStatus != "paid" {
		t.Fatal("earlier valid checkout ignored")
	}
	_, err = db.Collection("events").UpdateByID(context.Background(), e.ID, bson.M{"$set": bson.M{"checkoutId": "other", "checkoutIds": bson.A{}}})
	if err != nil {
		t.Fatal(err)
	}
	if w := deliver(); w.Code != 409 {
		t.Fatalf("unrecorded checkout reported processed: %d", w.Code)
	}
	r := httptest.NewRequest("POST", "/webhooks/stripe", strings.NewReader(body))
	r.Header.Set("Stripe-Signature", "invalid")
	w := httptest.NewRecorder()
	s.stripeWebhook(w, r)
	if w.Code != 400 {
		t.Fatal("invalid Stripe signature accepted")
	}
}

func TestConcurrentCheckoutRequestsShareProviderIdempotency(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_fake")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_fake")
	u := core.User{ID: "owner", Email: "owner@gmail.com", Role: "organizer"}
	if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("events").InsertOne(ctx, core.Event{ID: "event", Kind: "general", OwnerID: u.ID, PaymentStatus: "unpaid"}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	keys := map[string]bool{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				t.Error("missing provider idempotency")
			}
			mu.Lock()
			keys[key] = true
			mu.Unlock()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "checkout-same", "object": "checkout.session", "url": "https://stripe.test/session", "client_reference_id": "event", "payment_status": "unpaid", "status": "open"})
	}))
	defer provider.Close()
	previous := stripe.GetBackend(stripe.APIBackend)
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{URL: stripe.String(provider.URL), HTTPClient: provider.Client(), MaxNetworkRetries: stripe.Int64(0)}))
	defer stripe.SetBackend(stripe.APIBackend, previous)
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, err := s.sign(u, "password")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/events/event/checkout", nil)
			r.SetPathValue("id", "event")
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.checkout(w, r)
			if w.Code != 200 {
				t.Errorf("checkout: %d %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	if len(keys) != 1 {
		t.Fatalf("multiple provider sessions possible: %v", keys)
	}
	var saved struct {
		IDs []string `bson:"checkoutIds"`
	}
	if err = db.Collection("events").FindOne(ctx, bson.M{"_id": "event"}).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.IDs) != 1 || saved.IDs[0] != "checkout-same" {
		t.Fatal("checkout generations duplicated")
	}
}
