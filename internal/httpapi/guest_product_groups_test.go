package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuestCreationAndImportUseEventProductGroups(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	user := core.User{ID: "owner", Email: "owner@example.test", Role: "planner"}
	if _, err := db.Collection("users").InsertOne(ctx, user); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, err := s.sign(user, "password")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"general", "wedding"} {
		event := core.Event{ID: kind, Kind: kind, OwnerID: user.ID, PaymentStatus: "paid", Capacity: 100}
		if _, err := db.Collection("events").InsertOne(ctx, event); err != nil {
			t.Fatal(err)
		}
		valid, invalid := "Compañeros de estudios", "Familia del novio"
		if kind == "wedding" {
			valid, invalid = invalid, valid
		}
		for _, tc := range []struct {
			category string
			imported bool
			want     int
		}{{invalid, false, 400}, {valid, false, 201}, {invalid, true, 400}, {valid, true, 200}} {
			guest := guestInput{Name: "Ana", Phone: "+51987654321", Seats: 1, Family: tc.category, Gender: "unspecified"}
			var payload any = guest
			path := "/events/" + kind + "/guests"
			if tc.imported {
				payload = map[string]any{"guests": []guestInput{guest}}
				path += "/import"
			}
			body, _ := json.Marshal(payload)
			r := httptest.NewRequest("POST", path, strings.NewReader(string(body)))
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.routes().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("%s %s %v: %d %s", kind, tc.category, tc.imported, w.Code, w.Body.String())
			}
		}
	}
}
