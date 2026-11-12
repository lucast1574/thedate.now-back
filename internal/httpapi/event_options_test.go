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
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestDisabledOptionsGuardPublicLinksAndResponses(t *testing.T) {
	for _, kind := range []string{"general", "wedding"} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.Mongo(t)
			ctx := context.Background()
			now, disabled := time.Now().UTC(), false
			expires := now.Add(time.Hour)
			e := core.Event{ID: "event", Kind: kind, Slug: "celebracion", PaymentStatus: "paid", PublishedAt: &now, StartAt: now.Add(24 * time.Hour), Capacity: 4, MaybeHoldHours: 48, MaybeEnabled: &disabled, Celebration: &core.Celebration{GiftsEnabled: &disabled}, Wishlist: core.Wishlist{Items: []core.Gift{{ID: "gift"}}}, Responses: map[string]core.Response{"guest": {Choice: "maybe", Seats: 1, ExpiresAt: &expires}}, ResponseVersion: 1}
			g := core.Guest{ID: "guest", EventID: e.ID, Name: "Ana", Seats: 1, InviteToken: strings.Repeat("a", 48)}
			_, _ = db.Collection("events").InsertOne(ctx, e)
			_, _ = db.Collection("guests").InsertOne(ctx, g)
			s := &server{db: db}
			call := func(method, path, body string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				s.routes().ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
				return w
			}
			w := call("GET", "/public/rsvp/"+g.InviteToken, "")
			if w.Code != 200 || !strings.Contains(w.Body.String(), `"maybeEnabled":false`) || !strings.Contains(w.Body.String(), `"wishlistEnabled":false`) {
				t.Fatal(w.Code, w.Body.String())
			}
			w = call("POST", "/public/rsvp/"+g.InviteToken, `{"response":"maybe","reason":"Waiting for permission"}`)
			if w.Code != 409 || !strings.Contains(w.Body.String(), "maybe_disabled") {
				t.Fatal(w.Code, w.Body.String())
			}
			for _, path := range []string{"/public/wishlist/" + g.InviteToken, "/public/events/" + kind + "/celebracion/wishlist"} {
				if w = call("GET", path, ""); w.Code != 404 {
					t.Fatal("disabled gifts exposed", path, w.Code)
				}
			}
			if w = call("POST", "/public/events/"+kind+"/celebracion/wishlist/access", `{"phone":"+51999111222"}`); w.Code != 404 {
				t.Fatal("disabled gift access accepted", w.Code)
			}
			if w = call("POST", "/public/rsvp/"+g.InviteToken, `{"response":"going","companions":[]}`); w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			var saved core.Event
			_ = db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved)
			if saved.GiftFollowups[g.ID] != "" {
				t.Fatal("disabled gifts queued a message")
			}
			poll := invitationPoll(e, g)["poll"].(map[string]any)
			if len(poll["options"].([]string)) != 2 {
				t.Fatal("disabled maybe advertised in WhatsApp")
			}
		})
	}
}

func TestNewInvitationDefaultsMaybeOff(t *testing.T) {
	db := testutil.Mongo(t)
	u := core.User{ID: "owner", Email: "owner@example.test", Role: "user"}
	_, _ = db.Collection("users").InsertOne(context.Background(), u)
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, _ := s.sign(u, "password")
	in := core.EventInput{Kind: "general", Slug: "new-event", Title: "Our celebration", Organizer: "Host", StartAt: time.Now().Add(24 * time.Hour), Capacity: 10, Location: "Lima", MapURL: "https://maps.google.com/", Template: "classic", DesignMode: "flyer", TemplateID: "general-blank", AccentColor: "#7451ac"}
	body, _ := json.Marshal(in)
	r := httptest.NewRequest("POST", "/events", strings.NewReader(string(body)))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	var created core.Event
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &created) != nil || core.MaybeAllowed(created) {
		t.Fatal(w.Code, w.Body.String())
	}
}
