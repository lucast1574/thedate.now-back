package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"sync"
	"testing"
)

func TestConcurrentDefaultAffiliateActivationKeepsOneCodeAndWallet(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	repo := Affiliates{DB: db}
	if err := EnsureIndexes(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("users").InsertOne(ctx, bson.M{"_id": "account", "email": "account@example.test", "role": "organizer", "affiliateBalanceCents": int64(7000), "affiliateEarnedCents": int64(9000), "affiliateEntries": bson.M{"buyer": int64(9000)}}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	codes := make(chan string, 16)
	for range 16 {
		wg.Go(func() {
			w, err := repo.EnableDefaults(ctx, "account")
			if err != nil {
				t.Error(err)
				return
			}
			codes <- w.Code
		})
	}
	wg.Wait()
	close(codes)
	wallet, err := repo.Wallet(ctx, "account")
	if err != nil || !wallet.Enabled || len(wallet.Code) != 24 || wallet.Balance != 7000 || wallet.Earned != 9000 || wallet.Entries["buyer"] != 9000 {
		t.Fatalf("activation damaged wallet: %#v %v", wallet, err)
	}
	for code := range codes {
		if code != wallet.Code {
			t.Fatal("simultaneous activation replaced link")
		}
	}
}
