package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
)

func TestDeclinedGuestCannotReadInvitationOrUseWishlist(t *testing.T) {
	db := testutil.Mongo(t)
	now := time.Now()
	token := strings.Repeat("a", 48)
	e := core.Event{ID: "event", Kind: "wedding", Slug: "wedding", Title: "Celebration", PaymentStatus: "paid", PublishedAt: &now, Responses: map[string]core.Response{"guest": {Choice: "not_going"}}}
	g := core.Guest{ID: "guest", EventID: e.ID, Name: "Ana", InviteToken: token}
	if _, err := db.Collection("events").InsertOne(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("guests").InsertOne(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/public/rsvp/"+token, nil))
	var data map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || data["response"] != "not_going" || data["finalized"] != true {
		t.Fatalf("missing declined receipt: %s", w.Body.String())
	}
	event := data["event"].(map[string]any)
	if len(event) != 3 || event["id"] != e.ID || data["location"] != nil || data["virtualUrl"] != nil {
		t.Fatal("declined invitation exposes event details")
	}
	for _, method := range []string{"GET", "POST"} {
		w = httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest(method, "/public/wishlist/"+token, strings.NewReader(`{"giftId":"gift"}`)))
		if w.Code != 404 {
			t.Fatalf("%s wishlist allowed: %d", method, w.Code)
		}
	}
}
