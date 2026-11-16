package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMessageAssociationAcrossProductDatabases(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	secret := strings.Repeat("x", 48)
	var sent atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		reply(w, 200, map[string]string{"id": fmt.Sprintf("clarification-%d", sent.Load())})
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	for _, prefix := range []string{"WAZEND_GENERAL_", "WAZEND_WEDDING_"} {
		t.Setenv(prefix+"SESSION", "shared")
		t.Setenv(prefix+"WEBHOOK_HMAC", secret)
		t.Setenv(prefix+"BASE_URL", provider.URL)
		t.Setenv(prefix+"API_KEY", "test")
	}
	peers := []*server{{db: testutil.Mongo(t), product: "general", secret: []byte(secret)}, {db: testutil.Mongo(t), product: "wedding", secret: []byte(secret)}}
	for i, s := range peers {
		e := core.Event{ID: fmt.Sprintf("event-%d", i), Kind: s.product, OwnerID: "owner", PaymentStatus: "paid", PublishedAt: &now}
		g := core.Guest{ID: "guest", EventID: e.ID, Phone: "+51999111222", SentAt: &now, InvitationTextMessageID: fmt.Sprintf("invite-%d", i)}
		s.db.Collection("events").InsertOne(ctx, e)
		s.db.Collection("guests").InsertOne(ctx, g)
		if err := mongostore.EnsureInboxIndexes(ctx, s.db); err != nil {
			t.Fatal(err)
		}
	}
	handler := productRoutes(peers)
	post := func(id, text, quoted string) {
		t.Helper()
		payload := map[string]any{"id": id, "from": "51999111222@c.us", "body": text, "timestamp": now.Unix()}
		if quoted != "" {
			payload["replyTo"] = map[string]string{"id": quoted}
		}
		raw, _ := json.Marshal(map[string]any{"id": "webhook-" + id, "session": "shared", "event": "message", "payload": payload})
		mac := hmac.New(sha512.New, []byte(secret))
		mac.Write(raw)
		r := httptest.NewRequest("POST", "/webhooks/wazend", strings.NewReader(string(raw)))
		r.Header.Set("X-Webhook-Timestamp", fmt.Sprint(time.Now().UnixMilli()))
		r.Header.Set("X-Webhook-Hmac", hex.EncodeToString(mac.Sum(nil)))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("webhook %d %s", w.Code, w.Body)
		}
	}
	post("first", "¿A qué hora empieza?", "")
	post("first", "¿A qué hora empieza?", "")
	if sent.Load() != 1 {
		t.Fatal("clarification replay sent duplicate")
	}
	for _, s := range peers {
		n, _ := s.db.Collection("inboxMessages").CountDocuments(ctx, bson.M{"eventId": bson.M{"$regex": "^event-"}})
		if n != 0 {
			t.Fatal("unassociated content exposed")
		}
	}
	post("answer", "Para esta boda", "invite-1")
	for i, s := range peers {
		n, _ := s.db.Collection("inboxMessages").CountDocuments(ctx, bson.M{"eventId": fmt.Sprintf("event-%d", i)})
		expected := int64(0)
		if i == 1 {
			expected = 2
		}
		if n != expected {
			t.Fatalf("brand %d count %d", i, n)
		}
	}
	post("first", "¿A qué hora empieza?", "")
	// Responses and payments are never inferred from free text.
	var e core.Event
	peers[1].db.Collection("events").FindOne(ctx, bson.M{"_id": "event-1"}).Decode(&e)
	if len(e.Responses) != 0 || len(e.GuestPayments) != 0 {
		t.Fatal("message changed attendance or payments")
	}
}

func TestOrganizerReplyIsIdempotentAndPaymentsRejectWedding(t *testing.T) {
	ctx := context.Background()
	db := testutil.Mongo(t)
	secret := []byte(strings.Repeat("x", 48))
	s := &server{db: db, secret: secret}
	u := core.User{ID: "owner", Email: "owner@example.test", Role: "organizer"}
	db.Collection("users").InsertOne(ctx, u)
	e := core.Event{ID: "event", Kind: "general", OwnerID: u.ID, PaymentStatus: "paid"}
	db.Collection("events").InsertOne(ctx, e)
	db.Collection("guests").InsertOne(ctx, core.Guest{ID: "guest", EventID: e.ID, Phone: "+51999111222"})
	token, _ := s.sign(u, "password")
	var sends atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		reply(w, 200, map[string]string{"id": "sent-message"})
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	t.Setenv("WAZEND_GENERAL_BASE_URL", provider.URL)
	t.Setenv("WAZEND_GENERAL_API_KEY", "test")
	t.Setenv("WAZEND_GENERAL_SESSION", "session")
	requestID := uuid.NewString()
	body := fmt.Sprintf(`{"requestId":%q,"text":"Hola Ana"}`, requestID)
	for range 2 {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.SetPathValue("id", e.ID)
		r.SetPathValue("guestId", "guest")
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.replyConversation(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if sends.Load() != 1 {
		t.Fatal("retry re-sent message")
	}
	db.Collection("events").UpdateOne(ctx, bson.M{"_id": e.ID}, bson.M{"$set": bson.M{"kind": "wedding"}})
	r := httptest.NewRequest("GET", "/", nil)
	r.SetPathValue("id", e.ID)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.guestPayments(w, r)
	if w.Code != 404 {
		t.Fatal("payments exposed in wedding")
	}
}

func TestInboxRejectsUnsafeMediaAndNonDirectMessages(t *testing.T) {
	for _, raw := range []string{"https://evil.test/api/files/a", "http://wazend.test/api/files/a", "https://wazend.test@evil.test/api/files/a", "https://wazend.test/admin"} {
		if safeMessageMediaURL("https://wazend.test", raw) {
			t.Fatal("unsafe media", raw)
		}
	}
	if !safeMessageMediaURL("https://wazend.test", "https://wazend.test/api/files/a") {
		t.Fatal("valid media blocked")
	}
	for _, from := range []string{"status@broadcast", "123@g.us", "abc@c.us"} {
		var e wazendIncoming
		e.Payload.ID = "id"
		e.Payload.From = from
		e.Payload.Body = "hola"
		if _, ok := incomingMessage(e, time.Now()); ok {
			t.Fatal("non-direct message accepted")
		}
	}
}

func TestHiddenWhatsAppSenderResolvesThroughWazend(t *testing.T) {
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" || !strings.Contains(r.URL.Path, "/lids/") {
			t.Error("unexpected request")
		}
		reply(w, 200, map[string]string{"lid": "123@lid", "pn": "51999111222@c.us"})
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	from, err := resolveMessageSender(context.Background(), wazendSettings{Base: provider.URL, Key: "test-key", Session: "test"}, "123@lid")
	if err != nil || from != "51999111222@c.us" {
		t.Fatal("LID resolution failed", err)
	}
}
