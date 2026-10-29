package mongostore_test

import (
	"context"
	"fmt"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProfessionalRepeatedEventsHoldConcurrentReleaseAndRefund(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	repo := mongostore.Affiliates{DB: db}
	_, err := db.Collection("users").InsertOne(ctx, bson.M{"_id": "pro", "role": "planner", "affiliateBalanceCents": int64(0), "affiliateEarnedCents": int64(0)})
	if err != nil {
		t.Fatal(err)
	}
	q := core.CheckoutQuote{Amount: 2000, PartnerID: "pro", Benefit: 500, Mode: "client"}
	e := core.Event{ID: "first", Kind: "wedding", Title: "Boda", PaymentSource: "stripe", Payment: &core.Payment{SessionID: "cs", AmountCents: 2000, Live: true, PaidAt: now}}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := repo.ConvertPartner(ctx, e, q); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	other := e
	other.ID = "second"
	if err = repo.ConvertPartner(ctx, other, q); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.PartnerEntries(ctx, "pro", now); err != nil {
		t.Fatal(err)
	}
	w, _ := repo.Wallet(ctx, "pro")
	if w.Balance != 0 {
		t.Fatal("released early")
	}
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.PartnerEntries(ctx, "pro", now.Add(8*24*time.Hour))
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	w, _ = repo.Wallet(ctx, "pro")
	if w.Balance != 1000 || w.Earned != 1000 {
		t.Fatalf("double credit %+v", w)
	}
	e.Payment.RefundedCents = 2000
	if err = repo.ConvertPartner(ctx, e, q); err != nil {
		t.Fatal(err)
	}
	w, _ = repo.Wallet(ctx, "pro")
	if w.Balance != 500 {
		t.Fatal("refund")
	}
	e.Payment.RefundedCents = 0
	if err = repo.ConvertPartner(ctx, e, q); err != nil {
		t.Fatal(err)
	}
	_, _ = repo.PartnerEntries(ctx, "pro", now.Add(9*24*time.Hour))
	w, _ = repo.Wallet(ctx, "pro")
	if w.Balance != 500 {
		t.Fatal("refund replay resurrected money")
	}
}
func TestWalletConcurrentEventsCannotDoubleSpend(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	repo := mongostore.Affiliates{DB: db}
	now := time.Now().UTC()
	u := core.User{ID: "pro", Role: "organizer"}
	_, err := db.Collection("users").InsertOne(ctx, bson.M{"_id": u.ID, "role": u.Role, "affiliateBalanceCents": int64(800)})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var paid atomic.Int32
	for n := range 12 {
		e := core.Event{ID: fmt.Sprint(n), OwnerID: u.ID, Kind: "general", Slug: fmt.Sprint(n), PaymentStatus: "unpaid"}
		if _, err = db.Collection("events").InsertOne(ctx, e); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if repo.PayBalance(ctx, e, u, 800, now) == nil {
				paid.Add(1)
			}
		}()
	}
	wg.Wait()
	w, _ := repo.Wallet(ctx, u.ID)
	if w.Balance != 0 || paid.Load() != 1 {
		t.Fatalf("balance=%d paid=%d", w.Balance, paid.Load())
	}
	var e core.Event
	if err = db.Collection("events").FindOne(ctx, bson.M{"paymentStatus": "paid"}).Decode(&e); err != nil {
		t.Fatal(err)
	}
	e.PaymentStatus = "balance_pending"
	if err = repo.PayBalance(ctx, e, u, 800, now); err != nil {
		t.Fatal(err)
	}
	w, _ = repo.Wallet(ctx, u.ID)
	if w.Balance != 0 {
		t.Fatal("retry charged twice")
	}
}
func TestPaidAddressMigrationAllowsDraftsButRejectsTwoPaidClaims(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	_, err := db.Collection("events").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		t.Fatal(err)
	}
	if err = mongostore.EnsurePaidAddressIndex(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = mongostore.EnsurePaidAddressIndex(ctx, db); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		_, err = db.Collection("events").InsertOne(ctx, bson.M{"_id": id, "kind": "wedding", "slug": "same", "paymentStatus": "pending"})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Collection("events").UpdateByID(ctx, "a", bson.M{"$set": bson.M{"paymentStatus": "paid"}}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Collection("events").UpdateByID(ctx, "b", bson.M{"$set": bson.M{"paymentStatus": "paid"}})
	if !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("duplicate paid claim: %v", err)
	}
}

func TestCancelledBalanceAttemptCannotDebitAfterRetry(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	repo := mongostore.Affiliates{DB: db}
	u := core.User{ID: "organizer", Role: "organizer"}
	_, err := db.Collection("users").InsertOne(ctx, bson.M{"_id": u.ID, "affiliateBalanceCents": int64(800)})
	if err != nil {
		t.Fatal(err)
	}
	stale := core.Event{ID: "draft", OwnerID: u.ID, Kind: "general", Slug: "draft", PaymentStatus: "balance_pending", BalanceReservationID: "old", BalanceAmount: 800}
	if _, err = db.Collection("events").InsertOne(ctx, stale); err != nil {
		t.Fatal(err)
	}
	if err = repo.CancelBalance(ctx, stale, u); err != nil {
		t.Fatal(err)
	}
	if err = repo.PayBalance(ctx, stale, u, 800, time.Now()); err == nil {
		t.Fatal("cancelled reservation debited")
	}
	var fresh core.Event
	if err = db.Collection("events").FindOne(ctx, bson.M{"_id": stale.ID}).Decode(&fresh); err != nil {
		t.Fatal(err)
	}
	if err = repo.PayBalance(ctx, fresh, u, 800, time.Now()); err != nil {
		t.Fatal(err)
	}
	wallet, err := repo.Wallet(ctx, u.ID)
	if err != nil || wallet.Balance != 0 {
		t.Fatalf("wallet=%+v err=%v", wallet, err)
	}
}
