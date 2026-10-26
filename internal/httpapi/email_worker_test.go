package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type emailTestTransport func(*http.Request) (*http.Response, error)

func (f emailTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSupportEmailAtomicQueueAndConcurrentDelivery(t *testing.T) {
	s, tokens := supportFixture(t)
	t.Setenv("THE_DATE_RESEND_API_KEY", "local-test-key")
	ctx := context.Background()
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	var calls int
	var lock sync.Mutex
	http.DefaultClient = &http.Client{Transport: emailTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.resend.com/emails" || r.Header.Get("Idempotency-Key") == "" {
			t.Error("invalid provider request")
		}
		lock.Lock()
		calls++
		lock.Unlock()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":"test-accepted"}`))}, nil
	})}
	draft := supportDraft(t)
	w := supportCall(s, tokens["owner"], "POST", "/support/tickets", draft)
	var ticket core.SupportTicket
	if json.Unmarshal(w.Body.Bytes(), &ticket) != nil || w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	stored, err := (mongostore.SupportTickets{DB: s.db}).Get(ctx, ticket.ID, "", false)
	if err != nil || stored.Mail["ticket_received"].State != "pending" {
		t.Fatal("ticket and email were not saved together")
	}
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			err := s.deliverEmail(ctx, "supportTickets", "ticket_received")
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
	if calls != 1 {
		t.Fatalf("concurrent duplicate sends: %d", calls)
	}
	w = supportCall(s, tokens["admin"], "PATCH", "/admin/support/tickets/"+ticket.ID, map[string]any{"version": 0, "state": "DONE", "note": "Corregido"})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if err := s.deliverEmail(ctx, "supportTickets", "ticket_resolved"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("resolution notice missing")
	}
}

func TestEmailRetryRetainsSnapshotAndStableKey(t *testing.T) {
	s, _ := supportFixture(t)
	t.Setenv("THE_DATE_RESEND_API_KEY", "local-test-key")
	ctx := context.Background()
	_, err := s.db.Collection("users").UpdateOne(ctx, bson.M{"_id": "owner"}, bson.M{"$set": bson.M{"mail.welcome": core.PendingEmail(time.Now().UTC())}})
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = original })
	var bodies, keys []string
	http.DefaultClient = &http.Client{Transport: emailTestTransport(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		status, bodyText := 429, `{"error":"quota"}`
		if len(bodies) > 1 {
			status, bodyText = 200, `{"id":"test-accepted"}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(bodyText))}, nil
	})}
	if err = s.deliverEmail(ctx, "users", "welcome"); err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Collection("users").UpdateOne(ctx, bson.M{"_id": "owner"}, bson.M{"$set": bson.M{"name": "Changed after first attempt", "mail.welcome.nextAt": time.Now().Add(-time.Minute)}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.deliverEmail(ctx, "users", "welcome"); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] || keys[0] != keys[1] {
		t.Fatal("retry changed its persisted payload or key")
	}
}
