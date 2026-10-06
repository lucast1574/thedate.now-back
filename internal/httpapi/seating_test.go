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

func TestCouplePlansButOtherAccountsAndUnpaidEventsCannot(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	owner := core.User{ID: "owner", Email: "owner@example.com", Role: "planner"}
	couple := core.User{ID: "couple", Email: "couple@example.com", Role: "couple"}
	outsider := core.User{ID: "outside", Email: "outside@example.com", Role: "planner"}
	for _, u := range []core.User{owner, couple, outsider} {
		if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	e := core.Event{ID: "e", Kind: "wedding", OwnerID: owner.ID, CoupleUserIDs: []string{couple.ID}, Capacity: 3, PaymentStatus: "paid", PublishedAt: &now, MaybeHoldHours: 48}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	call := func(u core.User, method, path, body string) *httptest.ResponseRecorder {
		token, err := s.sign(u, "password")
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w
	}
	if w := call(outsider, "GET", "/events/e/seating", ""); w.Code != 404 {
		t.Fatal("outsider accessed plan")
	}
	w := call(couple, "POST", "/events/e/guests", `{"name":"Ana","lastName":"Rojas","phone":"+51987654321","seats":2,"gender":"woman","family":"Rojas"}`)
	if w.Code != 201 {
		t.Fatalf("couple add: %d %s", w.Code, w.Body.String())
	}
	var g core.Guest
	if err := json.Unmarshal(w.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}
	var stored core.Guest
	if err := db.Collection("guests").FindOne(ctx, bson.M{"_id": g.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	p := core.DefaultSeating()
	p.Tables = []core.SeatingTable{{ID: "t", Name: "Mesa 1", Shape: "round", Capacity: 2}}
	p.Assignments = map[string]string{g.ID + "~0": "t", g.ID + "~1": "t"}
	raw, _ := json.Marshal(p)
	w = call(couple, "PATCH", "/events/e/seating", string(raw))
	if w.Code != 200 {
		t.Fatalf("plan %d %s", w.Code, w.Body.String())
	}
	if w = call(owner, "PATCH", "/events/e/seating", string(raw)); w.Code != 409 {
		t.Fatal("stale plan not rejected")
	}
	w = call(core.User{}, "POST", "/public/rsvp/"+stored.InviteToken, `{"response":"going","companions":[{"name":"Luis","lastName":"Rojas","gender":"man"}]}`)
	// RSVP is token-based; the user header has no bearing on this public route.
	if w.Code != 200 {
		t.Fatalf("rsvp %d %s", w.Code, w.Body.String())
	}
	w = call(couple, "GET", "/events/e/guests", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"attendingSeats":2`) || !strings.Contains(w.Body.String(), `"name":"Luis"`) {
		t.Fatal("party details not returned")
	}
	w = call(core.User{}, "POST", "/public/rsvp/"+stored.InviteToken, `{"response":"going","companions":[{"name":"Luis"},{"name":"Pablo"}]}`)
	if w.Code != 400 {
		t.Fatal("extra companion accepted")
	}
	_, err := db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"paymentStatus": "unpaid"}})
	if err != nil {
		t.Fatal(err)
	}
	if w = call(couple, "GET", "/events/e/seating", ""); w.Code != 403 {
		t.Fatal("unpaid plan allowed")
	}
}
