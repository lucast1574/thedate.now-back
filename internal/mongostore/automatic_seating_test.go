package mongostore_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
)

func TestConcurrentConfirmedPairsGetAtomicCategoryTables(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "seating", Capacity: 40, PaymentStatus: "paid", PublishedAt: &now, ResponseVersion: 1, Responses: map[string]core.Response{}}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	guests := make([]core.Guest, 12)
	for n := range guests {
		category := "Amigos del novio"
		if n%2 == 0 {
			category = "Compañeros de trabajo de la novia"
		}
		guests[n] = core.Guest{ID: fmt.Sprintf("g%d", n), EventID: e.ID, Name: "Invitado", Seats: 2, Family: category, Response: "pending"}
		if _, err := db.Collection("guests").InsertOne(ctx, guests[n]); err != nil {
			t.Fatal(err)
		}
	}
	repo := mongostore.Responses{DB: db}
	s := application.Responses{Repository: repo, Now: func() time.Time { return now }}
	var wg sync.WaitGroup
	for _, g := range guests {
		wg.Add(1)
		go func(g core.Guest) {
			defer wg.Done()
			companions := []core.Person{{Name: "Pareja"}}
			if err := s.ApplyParty(ctx, e.ID, g, "going", "", "", &companions); err != nil {
				t.Error(err)
			}
		}(g)
	}
	wg.Wait()
	latest, err := repo.Snapshot(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Seating == nil || len(latest.Seating.Assignments) != 24 || len(latest.Seating.Tables) != 4 {
		t.Fatalf("bad plan: %+v", latest.Seating)
	}
	if err := core.ValidateSeating(*latest.Seating, latest, guests); err != nil {
		t.Fatal(err)
	}
	for _, table := range latest.Seating.Tables {
		category := ""
		for _, g := range guests {
			if latest.Seating.Assignments[g.ID+"~0"] == table.ID {
				if category != "" && category != g.Family {
					t.Fatal("mixed automatic categories")
				}
				category = g.Family
			}
		}
	}
	if err := s.Apply(ctx, e.ID, guests[0], "not_going", "", ""); err != nil {
		t.Fatal(err)
	}
	after, err := repo.Snapshot(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Seating.Assignments) != 22 {
		t.Fatal("decline did not release pair")
	}
	for id, table := range after.Seating.Assignments {
		if latest.Seating.Assignments[id] != table {
			t.Fatal("other guests reshuffled")
		}
	}
}
