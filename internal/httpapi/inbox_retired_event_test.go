package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRetiredEventKeepsExplicitMessageContextAcrossProducts(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]string{"id": "context-request"})
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	peers := []*server{{db: testutil.Mongo(t), product: "general"}, {db: testutil.Mongo(t), product: "wedding"}}
	for i, s := range peers {
		prefix := "WAZEND_GENERAL_"
		if s.product == "wedding" {
			prefix = "WAZEND_WEDDING_"
		}
		t.Setenv(prefix+"SESSION", "shared")
		t.Setenv(prefix+"BASE_URL", provider.URL)
		t.Setenv(prefix+"API_KEY", "test")
		for _, retired := range []bool{true, false} {
			id := fmt.Sprintf("%d-%t", i, retired)
			e := core.Event{ID: id, Kind: s.product, PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(48 * time.Hour), TimeZone: "UTC"}
			if retired {
				e.StartAt, e.AddressReleasedAt = now.Add(-48*time.Hour), &now
			}
			g := core.Guest{ID: id, EventID: id, Phone: "+51999111222", SentAt: &now, InvitationTextMessageID: "invitation-" + id}
			if _, err := s.db.Collection("events").InsertOne(ctx, e); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Collection("guests").InsertOne(ctx, g); err != nil {
				t.Fatal(err)
			}
		}
	}
	route := func(id, text, quoted string, media bool) core.InboxMessage {
		t.Helper()
		m := core.InboxMessage{ID: id, ProviderID: id, Session: "shared", Phone: "+51999111222", Text: text, ReplyTo: quoted, CreatedAt: now, ReceivedAt: now, Direction: "in"}
		if media {
			m.Media = &core.MessageMedia{MIME: "image/jpeg"}
		}
		if err := routeIncoming(ctx, peers, m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	assertOnly := func(id string, owner int, event string) {
		t.Helper()
		for i, s := range peers {
			var m core.InboxMessage
			err := s.db.Collection("inboxMessages").FindOne(ctx, bson.M{"_id": id}).Decode(&m)
			if i == owner {
				if err != nil || m.EventID != event || m.GuestID != event {
					t.Fatalf("wrong association for %s: %+v, %v", id, m, err)
				}
			} else if err == nil {
				t.Fatal("message leaked to other product", id)
			}
		}
	}
	for i := range peers {
		event := fmt.Sprintf("%d-true", i)
		id := "receipt-" + event
		route(id, "", "invitation-"+event, true)
		route(id, "", "invitation-"+event, true) // Provider retries keep the same destination.
		assertOnly(id, i, event)
		route("followup-"+event, "Pago pendiente", id, false)
		assertOnly("followup-"+event, i, event)
		route("code-"+event, core.EventMessageCode(event), "", false)
		assertOnly("code-"+event, i, event)
	}
	// Without explicit context, neither the active nor the retired events get it.
	route("ambiguous", "Te mando el comprobante", "", true)
	route("new-receipt", "", "invitation-0-true", true)
	assertOnly("new-receipt", 0, "0-true")
	for _, s := range peers {
		n, err := s.db.Collection("inboxMessages").CountDocuments(ctx, bson.M{"_id": "ambiguous", "eventId": bson.M{"$nin": bson.A{"", nil}}})
		if err != nil || n != 0 {
			t.Fatal("ambiguous message attached to an event", err)
		}
	}
	// A single remaining invitation needs no extra question, even when retired.
	for i, s := range peers {
		filter := bson.M{}
		if i == 0 {
			filter["eventId"] = bson.M{"$ne": "0-true"}
		}
		if _, err := s.db.Collection("guests").DeleteMany(ctx, filter); err != nil {
			t.Fatal(err)
		}
	}
	route("retired-no-context", "Hola", "", false)
	assertOnly("retired-no-context", 0, "0-true")
}
