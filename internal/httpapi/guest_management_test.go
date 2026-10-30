package httpapi

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOrganizerCorrectsClosedResponseAndRemovesGuest(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	u := core.User{ID: "owner", Email: "owner@example.test", Role: "user"}
	db.Collection("users").InsertOne(ctx, u)
	g := core.Guest{ID: "guest", EventID: "event", Name: "Ana", Phone: "+51999111222", Seats: 2, InviteToken: strings.Repeat("a", 48), SendState: "sent", SentAt: &now}
	prior := core.Response{Choice: "going", Seats: 2, Finalized: true, PartyRegistered: true, Companions: []core.Person{{Name: "Luis", LastName: "Perez", Gender: "man"}}}
	e := core.Event{ID: "event", Kind: "general", OwnerID: u.ID, PaymentStatus: "paid", PublishedAt: &now, Capacity: 2, MaybeHoldHours: 48, ResponseVersion: 1, Responses: map[string]core.Response{g.ID: prior}, Seating: &core.SeatingPlan{Assignments: map[string]string{"guest~0": "table", "guest~1": "table"}}, Wishlist: core.Wishlist{Items: []core.Gift{{ID: "gift", Title: "Regalo", GuestID: g.ID}}}}
	db.Collection("events").InsertOne(ctx, e)
	db.Collection("guests").InsertOne(ctx, g)
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, _ := s.sign(u, "password")
	action := func(method, body, auth string) int {
		r := httptest.NewRequest(method, "/", strings.NewReader(body))
		r.SetPathValue("id", e.ID)
		r.SetPathValue("guestId", g.ID)
		r.Header.Set("Authorization", "Bearer "+auth)
		w := httptest.NewRecorder()
		s.manageGuest(w, r)
		return w.Code
	}
	if action("PATCH", `{"response":"not_going"}`, "invalid") < 400 {
		t.Fatal("Unauthenticated correction accepted")
	}
	if code := action("PATCH", `{"response":"not_going"}`, token); code != 200 {
		t.Fatalf("Correction: %d", code)
	}
	current, _ := (mongostore.Responses{DB: db}).Snapshot(ctx, e.ID)
	if current.Responses[g.ID].Choice != "not_going" || core.Occupied(current, now) != 0 || len(current.Seating.Assignments) != 0 {
		t.Fatal("Correction did not release capacity and seating")
	}
	if code := action("PATCH", `{"response":"going"}`, token); code != 200 {
		t.Fatalf("Reconfirmation: %d", code)
	}
	current, _ = (mongostore.Responses{DB: db}).Snapshot(ctx, e.ID)
	if !core.ResponseCompleted(current.Responses[g.ID]) || len(current.Responses[g.ID].Companions) != 1 {
		t.Fatal("Party was lost during correction")
	}
	if code := action("DELETE", "", token); code != 200 {
		t.Fatalf("Removal: %d", code)
	}
	current, _ = (mongostore.Responses{DB: db}).Snapshot(ctx, e.ID)
	if core.Occupied(current, now) != 0 || len(current.Seating.Assignments) != 0 || current.Wishlist.Items[0].GuestID != "" || !current.RevokedGuestTokens[g.InviteToken] {
		t.Fatal("Removal left active related data")
	}
	if n, _ := db.Collection("guests").CountDocuments(ctx, bson.M{"_id": g.ID}); n != 0 {
		t.Fatal("Guest remains active")
	}
	if n, _ := db.Collection("deletedGuestBackups").CountDocuments(ctx, bson.M{"eventId": e.ID}); n != 1 {
		t.Fatal("Recovery copy missing")
	}
	if err := (s.responses()).ApplyProvider(ctx, e.ID, g, "going", "late", 0); !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("Stale response resurrected guest: %v", err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.SetPathValue("token", g.InviteToken)
	w := httptest.NewRecorder()
	s.rsvpDetails(w, r)
	if w.Code != 404 {
		t.Fatal("Removed link still active")
	}
	// Re-adding the same identity with a new token must not revive the old link or response.
	g.InviteToken = strings.Repeat("b", 48)
	g.SendState = ""
	g.SentAt = nil
	added, err := (mongostore.GuestImports{DB: db}).Add(ctx, g)
	if err != nil || !added {
		t.Fatal("Could not re-add guest", err)
	}
	service := application.ManagedResponses{Repository: mongostore.Responses{DB: db}, Now: func() time.Time { return now }}
	if err = service.Correct(ctx, e.ID, g, "going", ""); err != nil {
		t.Fatal("New token incorrectly revoked", err)
	}
}

func TestGuestCorrectionRespectsCapacityAndRemovalKeepsReceivedGifts(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	g := core.Guest{ID: "guest", EventID: "event", Seats: 2, InviteToken: strings.Repeat("c", 48), SendState: "sent", SendClaim: "original"}
	e := core.Event{ID: "event", PaymentStatus: "paid", PublishedAt: &now, Capacity: 1, ResponseVersion: 1, Responses: map[string]core.Response{g.ID: {Choice: "not_going", Finalized: true}}, Wishlist: core.Wishlist{Items: []core.Gift{{ID: "gift", GuestID: g.ID, Delivered: true}}}}
	db.Collection("events").InsertOne(ctx, e)
	db.Collection("guests").InsertOne(ctx, g)
	service := application.ManagedResponses{Repository: mongostore.Responses{DB: db}, Now: func() time.Time { return now }}
	if err := service.Correct(ctx, e.ID, g, "going", ""); !errors.Is(err, core.ErrCapacity) {
		t.Fatalf("Overbooking allowed: %v", err)
	}
	store := mongostore.GuestManagement{Responses: mongostore.Responses{DB: db}}
	if store.Remove(ctx, e.ID, g.ID, now) == nil {
		t.Fatal("Received gift silently removed")
	}
	var saved core.Guest
	db.Collection("guests").FindOne(ctx, bson.M{"_id": g.ID}).Decode(&saved)
	if saved.SendState != "sent" || saved.SendClaim != "original" {
		t.Fatal("Failed removal changed delivery state")
	}
	db.Collection("guests").UpdateByID(ctx, g.ID, bson.M{"$set": bson.M{"sendState": "sending"}})
	if !errors.Is(store.Remove(ctx, e.ID, g.ID, now), mongostore.ErrGuestSending) {
		t.Fatal("In-flight send deleted")
	}
}
