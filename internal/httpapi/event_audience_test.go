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

func TestAudienceStartsAtCreationAndSurvivesEventUpdates(t *testing.T) {
	for _, kind := range []string{"wedding", "general"} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.Mongo(t)
			ctx := context.Background()
			u := core.User{ID: "owner", Email: "owner@example.com", Role: "user"}
			if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
				t.Fatal(err)
			}
			s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
			token, err := s.sign(u, "password")
			if err != nil {
				t.Fatal(err)
			}
			in := core.EventInput{Kind: kind, Slug: "our-day", Title: "Our celebration", StartAt: time.Now().UTC().Add(time.Hour), TimeZone: "UTC", Organizer: "Host", Location: "Lima", MapURL: "https://maps.google.com/?q=Lima", Capacity: 20, MaybeHoldHours: 48, Template: "classic", AccentColor: "#766044", Celebration: &core.Celebration{AdultsOnly: true, AdultsMessage: "Celebración sin niños.", WhatsAppNotice: true, DressCode: "Elegante"}}
			call := func(method, path string, body core.EventInput) *httptest.ResponseRecorder {
				raw, _ := json.Marshal(body)
				r := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.routes().ServeHTTP(w, r)
				return w
			}
			w := call("POST", "/events", in)
			if w.Code != 201 {
				t.Fatalf("create %d %s", w.Code, w.Body.String())
			}
			var created core.Event
			if err = json.Unmarshal(w.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			if created.Celebration == nil || !created.Celebration.AdultsOnly || !created.Celebration.WhatsAppNotice {
				t.Fatal("creation lost audience")
			}
			in.Celebration.AdultsOnly = false
			w = call("PATCH", "/events/"+created.ID, in)
			if w.Code != 200 {
				t.Fatalf("update %d %s", w.Code, w.Body.String())
			}
			in.Celebration = nil
			w = call("PATCH", "/events/"+created.ID, in)
			if w.Code != 200 {
				t.Fatal("legacy update rejected")
			}
			var saved core.Event
			if err = db.Collection("events").FindOne(ctx, bson.M{"_id": created.ID}).Decode(&saved); err != nil {
				t.Fatal(err)
			}
			if saved.Celebration == nil || saved.Celebration.AdultsOnly || saved.Celebration.AdultsMessage != "Celebración sin niños." {
				t.Fatal("audience not persisted or erased by old client")
			}
			in.Celebration = &core.Celebration{AdultsMessage: strings.Repeat("x", 501)}
			if call("PATCH", "/events/"+created.ID, in).Code != 400 {
				t.Fatal("unbounded message accepted")
			}
		})
	}
}
