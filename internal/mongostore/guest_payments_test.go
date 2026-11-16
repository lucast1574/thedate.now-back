package mongostore_test

import (
	"context"
	"fmt"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentReceiptPaymentsAndAdmissionLock(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	a := &core.Admission{Enabled: true, Currency: "PEN", AmountCents: 8000, Basis: "person", Instructions: "Transferencia"}
	e := core.Event{ID: "event", Kind: "general", PaymentStatus: "paid", ResponseVersion: 1, Admission: a, StartAt: now.Add(24 * time.Hour), Capacity: 10, Template: "classic", AccentColor: "#abcdef", Title: "Cena", IsVirtual: true, VirtualURL: "https://example.com"}
	db.Collection("events").InsertOne(ctx, e)
	db.Collection("guests").InsertOne(ctx, core.Guest{ID: "guest", EventID: e.ID, Seats: 1})
	(mongostore.Inbox{DB: db}).Put(ctx, core.InboxMessage{ID: "photo", EventID: e.ID, GuestID: "guest", Direction: "in", Media: &core.MessageMedia{Available: true, Digest: "same-image-bytes"}})
	var wg sync.WaitGroup
	var recorded atomic.Int32
	for i := range 20 {
		wg.Go(func() {
			p := core.GuestPayment{ID: fmt.Sprintf("request-%d", i), GuestID: "guest", MessageID: "photo", AmountCents: 4000, Currency: "PEN", PaidOn: now.Format("2006-01-02"), Method: "transfer"}
			if mongostore.RecordGuestPayment(ctx, db, e, p, now) == nil {
				recorded.Add(1)
			}
		})
	}
	wg.Wait()
	var current core.Event
	db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&current)
	if recorded.Load() != 1 || len(current.GuestPayments) != 1 {
		t.Fatalf("duplicate money %d", recorded.Load())
	}
	for _, p := range current.GuestPayments {
		if err := mongostore.RecordGuestPayment(ctx, db, current, p, now); err != nil {
			t.Fatal("idempotent replay", err)
		}
	}
	in := core.EventInput{Kind: e.Kind, Title: e.Title, Organizer: "Organizador", Slug: "cena", StartAt: e.StartAt, IsVirtual: true, VirtualURL: e.VirtualURL, Capacity: 10, Template: e.Template, AccentColor: e.AccentColor, Admission: &core.Admission{Enabled: true, Currency: "USD", AmountCents: 2000, Basis: "person", Instructions: "Bank"}}
	err := (application.Responses{Repository: mongostore.Responses{DB: db}, Now: func() time.Time { return now }}).UpdateEvent(ctx, e.ID, in)
	if err != core.ErrAdmissionLocked {
		t.Fatalf("currency change allowed: %v", err)
	}
	foreign := current
	(mongostore.Inbox{DB: db}).Put(ctx, core.InboxMessage{ID: "repeated-photo", EventID: e.ID, GuestID: "guest", Direction: "in", Media: &core.MessageMedia{Available: true, Digest: "same-image-bytes"}})
	for _, p := range current.GuestPayments {
		p.ID = "second-photo-request"
		p.MessageID = "repeated-photo"
		if mongostore.RecordGuestPayment(ctx, db, current, p, now) == nil {
			t.Fatal("same image registered twice under different message IDs")
		}
	}
	foreign.ID = "other"
	for _, p := range current.GuestPayments {
		p.ID = "foreign"
		if mongostore.RecordGuestPayment(ctx, db, foreign, p, now) == nil {
			t.Fatal("cross-event receipt accepted")
		}
	}
}
