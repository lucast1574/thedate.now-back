package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestClosedEventRejectsEveryOwnerWriteAndKeepsReads(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	u := core.User{ID: "owner", Email: "owner@example.com", Role: "organizer"}
	if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
		t.Fatal(err)
	}
	token, err := s.sign(u, "password")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"general", "wedding"} {
		e := core.Event{ID: kind, Kind: kind, OwnerID: u.ID, StartAt: time.Now().AddDate(0, 0, -2), TimeZone: "America/Bogota", PaymentStatus: "paid"}
		if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
			t.Fatal(err)
		}
		for _, method := range []string{"POST", "PATCH", "DELETE", "GET"} {
			for _, suffix := range []string{"", "/design", "/design/publish", "/guests", "/guests/import", "/seating", "/wishlist", "/delivery", "/delivery/retry", "/photos", "/audio", "/collaborators", "/publish", "/checkout"} {
				called := false
				handler := s.eventWriteGuard(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) })
				r := httptest.NewRequest(method, "/events/"+e.ID+suffix, strings.NewReader(`{"startAt":"2099-01-01T00:00:00Z"}`))
				r.SetPathValue("id", e.ID)
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				handler(w, r)
				if method == "GET" {
					if !called || w.Code != 204 {
						t.Fatal("read blocked")
					}
				} else if called || w.Code != http.StatusConflict {
					t.Fatalf("write allowed %s %s: %d", method, suffix, w.Code)
				}
			}
		}
		if s.invitationSendOpen(ctx, e.ID) {
			t.Fatal("expired send remained enabled")
		}
		var stored core.Event
		if db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&stored) != nil || !stored.StartAt.Equal(e.StartAt.Truncate(time.Millisecond)) {
			t.Fatal("event changed")
		}
	}
}
