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

func TestSavedDraftIsPrivateUntilExplicitOwnerPublication(t *testing.T) {
	for _, kind := range []string{"general", "wedding"} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.Mongo(t)
			ctx := context.Background()
			now := time.Now().UTC()
			owner := core.User{ID: "owner", Role: "user", Email: "owner@example.com"}
			collaborator := core.User{ID: "collaborator", Role: "user", Email: "collaborator@example.com"}
			for _, u := range []core.User{owner, collaborator} {
				if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
					t.Fatal(err)
				}
			}
			e := core.Event{ID: "event", Kind: kind, Slug: "our-celebration", Title: "Public title", Template: "classic", AccentColor: "#112233", PaymentStatus: "paid", PublishedAt: &now, OwnerID: owner.ID, CoupleUserIDs: []string{collaborator.ID}, Sections: []core.Section{}, PhotoKeys: []string{}}
			if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
				t.Fatal(err)
			}
			s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
			request := func(user core.User, method, path, body string) *httptest.ResponseRecorder {
				t.Helper()
				r := httptest.NewRequest(method, path, strings.NewReader(body))
				if user.ID != "" {
					token, err := s.sign(user, "password")
					if err != nil {
						t.Fatal(err)
					}
					r.Header.Set("Authorization", "Bearer "+token)
				}
				w := httptest.NewRecorder()
				s.routes().ServeHTTP(w, r)
				return w
			}
			body := `{"title":"Private title","template":"classic","accentColor":"#112233","sections":[],"expectedVersion":0}`
			if w := request(collaborator, "PATCH", "/events/event/design", body); w.Code != 200 {
				t.Fatalf("save: %d %s", w.Code, w.Body.String())
			}
			if w := request(owner, "PATCH", "/events/event/design", body); w.Code != 409 {
				t.Fatalf("stale editor overwrite allowed: %d", w.Code)
			}
			checkPublic := func(title string) {
				t.Helper()
				w := request(core.User{}, "GET", "/public/events/"+kind+"/our-celebration", "")
				var view map[string]any
				_ = json.Unmarshal(w.Body.Bytes(), &view)
				if w.Code != 200 || view["title"] != title {
					t.Fatalf("public view: %d %s", w.Code, w.Body.String())
				}
				if _, exists := view["publishedDesign"]; exists {
					t.Fatal("snapshot leaked")
				}
			}
			checkPublic("Public title")
			if w := request(collaborator, "POST", "/events/event/design/publish", `{"expectedVersion":1}`); w.Code == 200 {
				t.Fatal("collaborator published")
			}
			if w := request(owner, "POST", "/events/event/design/publish", `{"expectedVersion":0}`); w.Code != 409 {
				t.Fatal("stale publication allowed")
			}
			if w := request(owner, "POST", "/events/event/design/publish", `{"expectedVersion":1}`); w.Code != 200 {
				t.Fatalf("publish: %d %s", w.Code, w.Body.String())
			}
			checkPublic("Private title")
			var stored core.Event
			if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&stored); err != nil {
				t.Fatal(err)
			}
			if stored.PublishedAt == nil || !stored.PublishedAt.Equal(now.Truncate(time.Millisecond)) || stored.Slug != e.Slug {
				t.Fatal("publication identity changed")
			}
		})
	}
}
