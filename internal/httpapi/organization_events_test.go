package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
)

func TestOrganizationEventsRequirePaymentAndPersonalAccessEvenForAdmin(t *testing.T) {
	db := testutil.Mongo(t)
	s := &server{db: db, product: "wedding", secret: []byte(strings.Repeat("s", 32))}
	u := core.User{ID: "admin", Email: "admin@gmail.com", Role: "admin", GoogleSub: "google", GoogleAuthoritative: true, IdentityPolicy: 1}
	if _, err := db.Collection("users").InsertOne(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	for _, e := range []core.Event{
		{ID: "own", Kind: "wedding", Slug: "own", OwnerID: u.ID, PaymentStatus: "paid"},
		{ID: "shared", Kind: "wedding", Slug: "shared", OwnerID: "other", CoupleUserIDs: []string{u.ID}, PaymentStatus: "paid"},
		{ID: "foreign", Kind: "wedding", Slug: "foreign", OwnerID: "other", PaymentStatus: "paid"},
		{ID: "pending", Kind: "wedding", Slug: "pending", OwnerID: u.ID, PaymentStatus: "pending"},
		{ID: "unpaid", Kind: "wedding", Slug: "unpaid", OwnerID: u.ID, PaymentStatus: "unpaid"},
		{ID: "demo", Kind: "wedding", Slug: "demo", OwnerID: u.ID, IsDemo: true, PaymentStatus: "paid"},
	} {
		if _, err := db.Collection("events").InsertOne(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	token, err := s.sign(u, "google")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/events?scope=organization", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.listEvents(w, r)
	if w.Code != 200 {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var events []core.Event
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two personal paid events, got %s", w.Body.String())
	}
	for _, e := range events {
		if e.ID != "own" && e.ID != "shared" {
			t.Fatalf("unexpected organization event: %s", e.ID)
		}
	}
}
