package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRSVPFinalizesOnceAcrossConcurrentRequestsAndReopensReadOnly(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now()
	token := strings.Repeat("a", 48)
	e := core.Event{ID: "event", Kind: "wedding", PaymentStatus: "paid", PublishedAt: &now, Capacity: 4, StartAt: now.Add(time.Hour)}
	g := core.Guest{ID: "guest", EventID: e.ID, Name: "Guest", Seats: 2, InviteToken: token}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("guests").InsertOne(ctx, g); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db}
	var saved, locked atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/public/rsvp/"+token, strings.NewReader(`{"response":"going","companions":[{"name":"Ana","lastName":"Lopez","gender":"woman"}]}`))
			r.SetPathValue("token", token)
			w := httptest.NewRecorder()
			s.rsvp(w, r)
			switch w.Code {
			case 200:
				saved.Add(1)
			case 409:
				locked.Add(1)
			default:
				t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	if saved.Load() != 1 || locked.Load() != 1 {
		t.Fatalf("saved=%d locked=%d", saved.Load(), locked.Load())
	}
	r := httptest.NewRequest("GET", "/public/rsvp/"+token, nil)
	r.SetPathValue("token", token)
	w := httptest.NewRecorder()
	s.rsvpDetails(w, r)
	var details struct {
		Finalized bool `json:"finalized"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &details); err != nil || !details.Finalized {
		t.Fatalf("reopen was not completed: %s", w.Body.String())
	}
	var updated core.Event
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Responses[g.ID].Finalized || updated.Responses[g.ID].Seats != 2 {
		t.Fatal("final party was not persisted")
	}
}
