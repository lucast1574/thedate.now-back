package mongostore_test

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"testing"
	"time"
)

func TestPlanAndResponseWritesFenceEachOther(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "e", PaymentStatus: "paid", PublishedAt: &now, Capacity: 3, ResponseVersion: 1, Responses: map[string]core.Response{}}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	repo := mongostore.Responses{DB: db}
	store := mongostore.Seating{DB: db}
	p := core.DefaultSeating()
	p.Tables = []core.SeatingTable{{ID: "t", Name: "Mesa", Capacity: 3, Shape: "round", X: 0, Y: 0}}
	p.Assignments = map[string]string{"g~0": "t", "g~1": "t"}
	ok, err := store.Save(ctx, e, p)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if ok, err = repo.CommitResponse(ctx, e, "g", core.Response{Seats: 1, Choice: "going"}, ""); err != nil || ok {
		t.Fatal("stale response overwrote seating")
	}
	current, err := repo.Snapshot(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	g := core.Guest{ID: "g", EventID: e.ID, Seats: 2}
	alone := []core.Person{}
	s := application.Responses{Repository: repo, Now: func() time.Time { return now }}
	if err = s.ApplyParty(ctx, e.ID, g, "going", "", "", &alone); err != nil {
		t.Fatal(err)
	}
	p.Version = 1
	if ok, err = store.Save(ctx, current, p); err != nil || ok {
		t.Fatal("stale plan overwrote RSVP")
	}
	latest, err := repo.Snapshot(ctx, e.ID)
	if err != nil || latest.Seating.Version != 2 || len(latest.Seating.Assignments) != 1 || latest.Responses[g.ID].Seats != 1 {
		t.Fatalf("bad reconciliation: %+v %v", latest, err)
	}
}
