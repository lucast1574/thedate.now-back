package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMaybeCanBecomeFinalWithoutReenteringCompanions(t *testing.T) {
	for _, channel := range []string{"web", "whatsapp"} {
		for _, choice := range []string{"going", "not_going"} {
			t.Run(channel+"/"+choice, func(t *testing.T) {
				db := testutil.Mongo(t)
				ctx := context.Background()
				now := time.Now()
				token := strings.Repeat("b", 48)
				e := core.Event{ID: "event", Kind: "wedding", PaymentStatus: "paid", PublishedAt: &now, Capacity: 2, MaybeHoldHours: 48, StartAt: now.Add(24 * time.Hour)}
				g := core.Guest{ID: "guest", EventID: e.ID, Name: "Guest", Seats: 2, InviteToken: token}
				if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Collection("guests").InsertOne(ctx, g); err != nil {
					t.Fatal(err)
				}
				s := &server{db: db}
				post := func(body string) *httptest.ResponseRecorder {
					r := httptest.NewRequest("POST", "/public/rsvp/"+token, strings.NewReader(body))
					r.SetPathValue("token", token)
					w := httptest.NewRecorder()
					s.rsvp(w, r)
					return w
				}
				body := `{"response":"maybe","reason":"Esperando permiso","companions":[{"name":"Ana","lastName":"Lopez","gender":"woman"}]}`
				if w := post(body); w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
				var saved core.Event
				if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved); err != nil {
					t.Fatal(err)
				}
				first := saved.Responses[g.ID]
				if first.Finalized || core.ResponseCompleted(first) {
					t.Fatal("maybe was closed")
				}
				if w := post(body); w.Code != 200 {
					t.Fatal("maybe could not be saved again", w.Code)
				}
				if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved); err != nil {
					t.Fatal(err)
				}
				if !saved.Responses[g.ID].ExpiresAt.Equal(*first.ExpiresAt) {
					t.Fatal("reservation restarted")
				}
				if err := s.responses().ApplyProvider(ctx, e.ID, g, "maybe", "repeat-maybe", 90); err != nil {
					t.Fatal(err)
				}
				if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved); err != nil {
					t.Fatal(err)
				}
				if saved.Responses[g.ID].Reason != "Esperando permiso" {
					t.Fatal("repeat vote cleared the reason")
				}
				// Simulate a maybe saved under the earlier version: it must recover without a migration.
				if _, err := db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"responses." + g.ID + ".finalized": true}}); err != nil {
					t.Fatal(err)
				}
				if channel == "web" {
					if w := post(`{"response":"` + choice + `"}`); w.Code != 200 {
						t.Fatal(w.Code, w.Body.String())
					}
				} else {
					if err := s.responses().ApplyProvider(ctx, e.ID, g, choice, "poll-final", 100); err != nil {
						t.Fatal(err)
					}
				}
				if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved); err != nil {
					t.Fatal(err)
				}
				response := saved.Responses[g.ID]
				if response.Choice != choice || !core.ResponseCompleted(response) || len(response.Companions) != 1 || response.Companions[0].Name != "Ana" {
					t.Fatal("final confirmation lost its party")
				}
				r := httptest.NewRequest("GET", "/public/rsvp/"+token, nil)
				r.SetPathValue("token", token)
				w := httptest.NewRecorder()
				s.rsvpDetails(w, r)
				var details struct {
					Finalized bool `json:"finalized"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &details); err != nil || !details.Finalized {
					t.Fatal("terminal response did not close")
				}
				if w := post(body); w.Code != 409 {
					t.Fatal("terminal response could be reopened", w.Code)
				}
			})
		}
	}
}
