package mongostore_test

import (
	"context"
	"fmt"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAffiliateFirstPaymentReplayRefundAndWithdrawalConcurrency(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	repo := mongostore.Affiliates{DB: db}
	for _, u := range []bson.M{{"_id": "affiliate", "affiliateEnabled": true, "affiliateBalanceCents": int64(0)}, {"_id": "referred", "referredBy": "affiliate"}} {
		if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	e := core.Event{ID: "first", OwnerID: "referred", PaymentSource: "stripe", Payment: &core.Payment{SessionID: "paid", AmountCents: 100000, Currency: "USD", Live: true, PaidAt: time.Now()}}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if err := repo.Convert(ctx, e.OwnerID, e); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	wallet, err := repo.Wallet(ctx, "affiliate")
	if err != nil || wallet.Balance != 10000 || wallet.Earned != 10000 {
		t.Fatalf("duplicate commission: %#v %v", wallet, err)
	}
	second := e
	second.Payment = &core.Payment{SessionID: "second", AmountCents: 100000, Live: true}
	if err := repo.Convert(ctx, e.OwnerID, second); err != nil {
		t.Fatal(err)
	}
	wallet, _ = repo.Wallet(ctx, "affiliate")
	if wallet.Balance != 10000 {
		t.Fatal("second payment credited")
	}
	courtesy := e
	courtesy.PaymentSource = "courtesy"
	if err := repo.Convert(ctx, e.OwnerID, courtesy); err != nil {
		t.Fatal(err)
	}
	refunded := e
	refunded.Payment = &core.Payment{SessionID: "paid", AmountCents: 100000, RefundedCents: 50000, Live: true}
	for range 8 {
		wg.Go(func() {
			if err := repo.Convert(ctx, e.OwnerID, refunded); err != nil {
				t.Error(err)
			}
		})
		wg.Go(func() {
			if err := repo.Convert(ctx, e.OwnerID, e); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	wallet, _ = repo.Wallet(ctx, "affiliate")
	if wallet.Balance != 5000 || wallet.Earned != 5000 {
		t.Fatalf("refund replay resurrected credit: %#v", wallet)
	}
	var reserved atomic.Int32
	for i := range 12 {
		wg.Go(func() {
			if repo.Reserve(ctx, "affiliate", core.Withdrawal{ID: fmt.Sprintf("request-%d", i), AmountCents: 5000, Status: "pending"}) == nil {
				reserved.Add(1)
			}
		})
	}
	wg.Wait()
	if reserved.Load() != 1 {
		t.Fatal("parallel withdrawal overspent balance")
	}
	wallet, _ = repo.Wallet(ctx, "affiliate")
	if wallet.Balance != 0 {
		t.Fatal("reservation not deducted")
	}
	for id := range wallet.Withdrawals {
		for range 8 {
			wg.Go(func() { _ = repo.Transition(ctx, "affiliate", id, "rejected", "test") })
		}
	}
	wg.Wait()
	wallet, _ = repo.Wallet(ctx, "affiliate")
	if wallet.Balance != 5000 {
		t.Fatal("rejected withdrawal released more than once")
	}
}
func TestAffiliateTestsNeverCreateWithdrawableCredit(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	repo := mongostore.Affiliates{DB: db}
	_, _ = db.Collection("users").InsertOne(ctx, bson.M{"_id": "affiliate", "affiliateEnabled": true})
	_, _ = db.Collection("users").InsertOne(ctx, bson.M{"_id": "buyer", "referredBy": "affiliate"})
	e := core.Event{ID: "test", PaymentSource: "stripe", Payment: &core.Payment{SessionID: "test", AmountCents: 100000, Live: false}}
	if err := repo.Convert(ctx, "buyer", e); err != nil {
		t.Fatal(err)
	}
	wallet, _ := repo.Wallet(ctx, "affiliate")
	if wallet.Balance != 0 || wallet.Earned != 0 {
		t.Fatal("test money credited")
	}
	e.Payment.Live = true
	e.Payment.SessionID = "real"
	if err := repo.Convert(ctx, "buyer", e); err != nil {
		t.Fatal(err)
	}
	wallet, _ = repo.Wallet(ctx, "affiliate")
	if wallet.Balance != 10000 {
		t.Fatal("test consumed first real purchase")
	}
}
