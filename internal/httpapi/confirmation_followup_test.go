package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
)

func TestManualConfirmationSendsIndependentMessagesAndRecognizesReceipts(t *testing.T) {
	for _, kind := range []string{"general", "wedding"} {
		t.Run(kind, func(t *testing.T) {
			ctx, now := context.Background(), time.Now().UTC()
			db := testutil.Mongo(t)
			s := &server{db: db, product: kind, secret: []byte(strings.Repeat("x", 48))}
			owner := core.User{ID: "owner", Email: "owner@example.test", Role: "organizer"}
			g := core.Guest{ID: "guest", EventID: "event", Name: "Ana", Phone: "+51999111222", Seats: 2, InviteToken: strings.Repeat("a", 48), SentAt: &now}
			e := core.Event{ID: g.EventID, OwnerID: owner.ID, Kind: kind, Slug: "celebracion", Title: "Celebración", Capacity: 10, PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(24 * time.Hour), ResponseVersion: 1, Responses: map[string]core.Response{}}
			if kind == "general" {
				e.Admission = &core.Admission{Enabled: true, Currency: "PEN", Basis: "person", AmountCents: 4000}
			}
			for name, record := range map[string]any{"events": e, "guests": g, "users": owner} {
				if _, err := db.Collection(name).InsertOne(ctx, record); err != nil {
					t.Fatal(err)
				}
			}
			var sends atomic.Int32
			provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reply(w, 200, map[string]string{"id": "out-" + strconv.Itoa(int(sends.Add(1)))})
			}))
			defer provider.Close()
			previous := wazendClient
			wazendClient = provider.Client()
			defer func() { wazendClient = previous }()
			prefix := "WAZEND_" + strings.ToUpper(kind) + "_"
			t.Setenv(prefix+"BASE_URL", provider.URL)
			t.Setenv(prefix+"API_KEY", "test")
			t.Setenv(prefix+"SESSION", "shared")
			token, _ := s.sign(owner, "password")
			request := func(method, path, body string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, path, strings.NewReader(body))
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.routes().ServeHTTP(w, r)
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
				return w
			}
			guestPath := "/events/event/guests/guest"
			request("PATCH", guestPath, `{"response":"going"}`)
			request("PATCH", guestPath, `{"response":"not_going"}`)
			s.confirmationFollowupStep(ctx, e.ID)
			if sends.Load() != 0 {
				t.Fatal("sent after a decline")
			}
			request("PATCH", guestPath, `{"response":"going"}`)
			for range 2 {
				s.confirmationFollowupStep(ctx, e.ID)
			}
			want := []string{"companions", "gifts"}
			if kind == "general" {
				want = append(want, "payment")
			}
			if int(sends.Load()) != len(want) {
				t.Fatal("messages lost or duplicated", sends.Load(), want)
			}
			list := request("GET", "/events/event/conversations", "")
			var conversations []core.Conversation
			if err := json.Unmarshal(list.Body.Bytes(), &conversations); err != nil || len(conversations) != 1 || conversations[0].Unread || conversations[0].Pending {
				t.Fatal("automatic conversation has wrong inbox status", list.Body.String())
			}
			if kind == "general" {
				for _, quoted := range []string{"", "out-3"} {
					m := core.InboxMessage{ID: "in-" + quoted, ProviderID: "received-" + quoted, Session: "shared", Phone: g.Phone, Direction: "in", CreatedAt: now, ReceivedAt: now, Media: &core.MessageMedia{MIME: "application/pdf"}, ReplyTo: quoted}
					if err := routeIncoming(ctx, []*server{s}, m); err != nil {
						t.Fatal(err)
					}
				}
			}
			w := request("GET", "/events/event/conversations/guest", "")
			var page struct{ Messages []core.InboxMessage }
			if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, message := range page.Messages {
				if message.Automation != "" {
					got = append(got, message.Automation)
				}
				switch message.ID {
				case "in-":
					if message.ReceiptReview != "review" || message.PaymentRecorded {
						t.Fatal("direct receipt was not marked for review", message)
					}
				case "in-out-3":
					if message.ReceiptReview != "possible" || message.PaymentRecorded {
						t.Fatal("quoted receipt was not identified", message)
					}
				}
			}
			slices.Reverse(got)
			if !slices.Equal(got, want) || (kind == "general" && len(page.Messages) != 5) {
				t.Fatal("unexpected history", w.Body.String())
			}
		})
	}
}
