package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSharedWebhookUpdatesOnlyTheMatchingProduct(t *testing.T) {
	key := strings.Repeat("h", 40)
	t.Setenv("WAZEND_WEBHOOK_HMAC", key)
	t.Setenv("WAZEND_SESSION", "test-session")
	servers := []*server{{product: "general", db: testutil.Mongo(t)}, {product: "wedding", db: testutil.Mongo(t)}}
	now := time.Now().UTC()
	for _, s := range servers {
		e := core.Event{ID: "event-" + s.product, Kind: s.product, Capacity: 10, PaymentStatus: "paid", PublishedAt: &now}
		messageID := "message-" + s.product
		if s.product == "general" {
			messageID = "true_51999111222@c.us_" + messageID
		}
		g := core.Guest{ID: "guest-" + s.product, EventID: e.ID, Seats: 1, Response: "pending", InvitationMessageID: messageID}
		if _, err := s.db.Collection("events").InsertOne(context.Background(), e); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Collection("guests").InsertOne(context.Background(), g); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"wedding", "general"} {
		body := fmt.Sprintf(`{"id":"response-%s","session":"test-session","event":"event.response","payload":{"eventCreationKey":{"id":"false_51999888777@c.us_message-%s"},"eventResponse":{"response":"GOING"}}}`, target, target)
		mac := hmac.New(sha512.New, []byte(key))
		mac.Write([]byte(body))
		r := httptest.NewRequest("POST", "/webhooks/wazend", strings.NewReader(body))
		r.Header.Set("X-Webhook-Timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
		r.Header.Set("X-Webhook-Hmac", hex.EncodeToString(mac.Sum(nil)))
		w := httptest.NewRecorder()
		sharedWebhook(servers, "wazend").ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("webhook: %d %s", w.Code, w.Body.String())
		}
		for _, s := range servers {
			var e core.Event
			if err := s.db.Collection("events").FindOne(context.Background(), bson.M{"_id": "event-" + s.product}).Decode(&e); err != nil {
				t.Fatal(err)
			}
			if s.product == target && e.Responses["guest-"+target].Choice != "going" {
				t.Fatal("matching product did not receive response")
			}
			if s.product != target && e.Responses["guest-"+target].Choice != "" {
				t.Fatal("response leaked into another product")
			}
		}
	}
}

func TestSharedWebhookStillRejectsInvalidSignatures(t *testing.T) {
	t.Setenv("WAZEND_WEBHOOK_HMAC", strings.Repeat("h", 40))
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_example")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_example")
	servers := []*server{{product: "general"}, {product: "wedding"}}
	for _, provider := range []string{"stripe", "wazend"} {
		r := httptest.NewRequest("POST", "/webhooks/"+provider, strings.NewReader(`{}`))
		r.Header.Set("X-Webhook-Timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
		w := httptest.NewRecorder()
		sharedWebhook(servers, provider).ServeHTTP(w, r)
		if w.Code < 400 {
			t.Fatalf("unsigned %s webhook accepted", provider)
		}
	}
}
