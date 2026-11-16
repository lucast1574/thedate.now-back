package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestContextPollAssignsOnlyItsMessageWithoutChangingAttendance(t *testing.T) {
	ctx, now := context.Background(), time.Now().UTC()
	secret := strings.Repeat("x", 48)
	polls := []map[string]any{}
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if r.URL.Path != "/api/sendPoll" || json.NewDecoder(r.Body).Decode(&payload) != nil {
			t.Error("expected native poll")
		}
		polls = append(polls, payload)
		raw, _ := json.Marshal(payload)
		for _, forbidden := range []string{"código", "TD-", "pago", "transferencia"} {
			if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(forbidden)) {
				t.Error("technical or payment prompt", string(raw))
			}
		}
		reply(w, 200, map[string]string{"id": fmt.Sprintf("poll-%d", len(polls))})
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	peers := []*server{{db: testutil.Mongo(t), product: "general"}, {db: testutil.Mongo(t), product: "wedding"}}
	for i, s := range peers {
		prefix := "WAZEND_GENERAL_"
		if i == 1 {
			prefix = "WAZEND_WEDDING_"
		}
		t.Setenv(prefix+"BASE_URL", provider.URL)
		t.Setenv(prefix+"API_KEY", "test")
		t.Setenv(prefix+"SESSION", "shared")
		t.Setenv(prefix+"WEBHOOK_HMAC", secret)
		e := core.Event{ID: fmt.Sprint(i), Kind: s.product, Title: "Celebración", StartAt: now.Add(-48 * time.Hour), AddressReleasedAt: &now, PublishedAt: &now, PaymentStatus: "paid"}
		g := core.Guest{ID: "guest", EventID: e.ID, Phone: "+51999111222", SentAt: &now}
		if _, err := s.db.Collection("events").InsertOne(ctx, e); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Collection("guests").InsertOne(ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	handler := productRoutes(peers)
	for _, id := range []string{"photo-a", "photo-b", "photo-a"} {
		m := core.InboxMessage{ID: id, ProviderID: id, Session: "shared", Phone: "+51999111222", Direction: "in", CreatedAt: now, ReceivedAt: now, Media: &core.MessageMedia{MIME: "image/jpeg"}}
		if err := routeIncoming(ctx, peers, m); err != nil {
			t.Fatal(err)
		}
	}
	if len(polls) != 2 {
		t.Fatal("poll send was duplicated", len(polls))
	}
	var a, b core.InboxMessage
	peers[0].db.Collection("inboxMessages").FindOne(ctx, bson.M{"_id": "photo-a"}).Decode(&a)
	peers[0].db.Collection("inboxMessages").FindOne(ctx, bson.M{"_id": "photo-b"}).Decode(&b)
	if len(a.ContextChoices) != 2 || a.ContextChoices[0].Label == a.ContextChoices[1].Label {
		t.Fatal("event labels not distinct")
	}
	vote := func(id, option, from string) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"id": "vote-" + id, "event": "poll.vote", "session": "shared", "payload": map[string]any{"poll": map[string]any{"id": id, "fromMe": true}, "vote": map[string]any{"from": from, "selectedOptions": []string{option}, "timestamp": now.Unix()}}})
		mac := hmac.New(sha512.New, []byte(secret))
		mac.Write(raw)
		r := httptest.NewRequest("POST", "/webhooks/wazend", strings.NewReader(string(raw)))
		r.Header.Set("X-Webhook-Timestamp", fmt.Sprint(now.UnixMilli()))
		r.Header.Set("X-Webhook-Hmac", hex.EncodeToString(mac.Sum(nil)))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	vote(a.ContextPolls[0].ID, a.ContextChoices[1].Label, "51999888777@c.us")
	var unchanged core.InboxMessage
	peers[0].db.Collection("inboxMessages").FindOne(ctx, bson.M{"_id": a.ID}).Decode(&unchanged)
	if unchanged.EventID != "" {
		t.Fatal("another voter assigned the image")
	}
	for range 2 {
		vote(a.ContextPolls[0].ID, a.ContextChoices[1].Label, "51999111222@c.us")
	}
	vote(a.ContextPolls[0].ID, a.ContextChoices[0].Label, "51999111222@c.us")
	vote(b.ContextPolls[0].ID, b.ContextChoices[0].Label, "51999111222@c.us")
	for i, s := range peers {
		id := "photo-b"
		if i == 1 {
			id = "photo-a"
		}
		var m core.InboxMessage
		if err := s.db.Collection("inboxMessages").FindOne(ctx, bson.M{"_id": id, "eventId": fmt.Sprint(i), "guestId": "guest"}).Decode(&m); err != nil {
			t.Fatal("wrong destination", err)
		}
		var e core.Event
		s.db.Collection("events").FindOne(ctx, bson.M{"_id": fmt.Sprint(i)}).Decode(&e)
		if len(e.Responses) != 0 || len(e.GuestPayments) != 0 {
			t.Fatal("event selection changed attendance or payments")
		}
	}
	if len(polls) != 2 {
		t.Fatal("vote started another prompt")
	}
}

func TestContextPollPagesKeepEveryEventAndNoTechnicalCodes(t *testing.T) {
	choices := make([]core.MessageContextChoice, 25)
	for i := range choices {
		choices[i].Label = fmt.Sprint(i)
	}
	for page, want := range []int{11, 11, 3} {
		visible, more := contextPage(choices, page)
		if len(visible) != want || more != (page < 2) {
			t.Fatal("invalid poll page", page)
		}
	}
	e := core.Event{Kind: "general", Title: "Cumpleaños Lucas", Slug: "cumpleanoslucas"}
	text := eventMessage(e, core.Guest{})["text"].(string)
	if strings.Contains(text, "Código") || strings.Contains(text, "TD-") || strings.Contains(text, "comprobante") {
		t.Fatal("free invitation includes technical/payment instructions", text)
	}
}
