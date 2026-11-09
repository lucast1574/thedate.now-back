package mongostore_test

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"testing"
	"time"
)

func TestGiftFollowupClaimGuardsAndUnknownOutcome(t *testing.T) {
	for _, choice := range []string{"maybe", "not_going", "going"} {
		t.Run(choice, func(t *testing.T) {
			db := testutil.Mongo(t)
			ctx := context.Background()
			now := time.Now().UTC()
			e := core.Event{ID: "e", PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(time.Hour), ResponseVersion: 1, Responses: map[string]core.Response{"g": {Choice: choice}}, GiftFollowups: map[string]string{"g": "pending"}}
			g := core.Guest{ID: "g", EventID: e.ID, InviteToken: "token"}
			if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
				t.Fatal(err)
			}
			repo := mongostore.Responses{DB: db}
			ok, err := repo.ClaimGiftFollowup(ctx, e, g, now)
			if err != nil || ok != (choice == "going") {
				t.Fatal(ok, err)
			}
			if choice != "going" {
				return
			}
			if err := repo.FinishGiftFollowup(ctx, e.ID, g.ID, "unknown"); err != nil {
				t.Fatal(err)
			}
			ok, err = repo.ClaimGiftFollowup(ctx, e, g, now)
			if err != nil || ok {
				t.Fatal("unknown outcome retried", err)
			}
			// A response saved using the pre-claim snapshot cannot reset the claim.
			ok, err = repo.CommitResponse(ctx, e, g.ID, core.Response{Choice: "going", Seats: 1}, "")
			if err != nil || ok {
				t.Fatal("stale response overwrote claim", err)
			}
		})
	}
}
