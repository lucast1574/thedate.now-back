package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProfessionalUpgradeAndDemoActivation(t *testing.T) {
	for _, kind := range []string{"general", "wedding"} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.Mongo(t)
			ctx := context.Background()
			s := &server{db: db, product: kind, secret: []byte(strings.Repeat("x", 32))}
			if err := mongostore.EnsureIndexes(ctx, db); err != nil {
				t.Fatal(err)
			}
			u := core.User{ID: "owner", Product: kind, Role: "user", Email: "owner@example.test", Name: "Owner"}
			db.Collection("users").InsertOne(ctx, u)
			token, _ := s.sign(u, "password")
			request := func(method, path string, body any) *httptest.ResponseRecorder {
				raw, _ := json.Marshal(body)
				r := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.routes().ServeHTTP(w, r)
				return w
			}
			paid := core.Event{ID: "paid", Kind: kind, Slug: "paid", OwnerID: u.ID, PaymentStatus: "paid"}
			db.Collection("events").InsertOne(ctx, paid)
			if w := request("GET", "/events/paid/collaborators", nil); w.Code != 403 {
				t.Fatal("ordinary role can invite", w.Code)
			}
			if w := request("POST", "/profile/professional", nil); w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			if w := request("GET", "/events/paid/collaborators", nil); w.Code != 200 {
				t.Fatal("professional access denied", w.Code, w.Body.String())
			}
			var stored core.User
			db.Collection("users").FindOne(ctx, bson.M{"_id": u.ID}).Decode(&stored)
			p, _ := core.ProductFor(kind)
			if stored.Role != p.Role || stored.TokenVersion != 0 {
				t.Fatal("wrong professional role or session invalidated")
			}
			demo := core.Event{ID: "demo", Kind: kind, Slug: "demo", OwnerID: u.ID, IsDemo: true, PaymentStatus: "demo", Template: "classic", TemplateID: core.TemplateID(kind, "classic", "sections", ""), AccentColor: "#996644", PhotoKeys: []string{"my-photo.jpg"}, AudioKeys: []string{"my-song.mp3"}, Sections: []core.Section{{ID: "custom", Icon: "none", Heading: "My own canvas", Body: "My own text"}}}
			db.Collection("events").InsertOne(ctx, demo)
			in := core.EventInput{Kind: kind, Slug: "our-event", Title: "Our event", StartAt: time.Now().Add(time.Hour), Organizer: "Owner", Location: "Lima", MapURL: "https://maps.google.com/", Capacity: 50, MaybeHoldHours: 48, Template: "classic", TemplateID: demo.TemplateID, AccentColor: "#996644"}
			var group sync.WaitGroup
			results := make(chan *httptest.ResponseRecorder, 2)
			for range 2 {
				group.Add(1)
				go func() { defer group.Done(); results <- request("POST", "/events/demo/activate", in) }()
			}
			group.Wait()
			close(results)
			for w := range results {
				if w.Code != 200 && w.Code != 409 {
					t.Fatal(w.Code, w.Body.String())
				}
			}
			var activated core.Event
			db.Collection("events").FindOne(ctx, bson.M{"_id": "demo"}).Decode(&activated)
			if activated.IsDemo || activated.PaymentStatus != "unpaid" || activated.Slug != "our-event" || len(activated.Sections) != 1 || activated.Sections[0].Body != "My own text" || activated.PhotoKeys[0] != "my-photo.jpg" || activated.AudioKeys[0] != "my-song.mp3" {
				t.Fatal("demo work lost or free payment forged")
			}
		})
	}
}
func TestRoleMigrationProtectsAdminAndMembership(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	for _, u := range []core.User{{ID: "root", Role: "admin", TokenVersion: 5}, {ID: "legacy", Role: "couple"}, {ID: "planner", Role: "planner"}} {
		db.Collection("users").InsertOne(ctx, u)
	}
	if err := mongostore.EnsureProductRoles(ctx, db, "wedding"); err != nil {
		t.Fatal(err)
	}
	var root, legacy core.User
	db.Collection("users").FindOne(ctx, bson.M{"_id": "root"}).Decode(&root)
	db.Collection("users").FindOne(ctx, bson.M{"_id": "legacy"}).Decode(&legacy)
	if root.Role != "admin" || root.TokenVersion != 5 || legacy.Role != "user" || legacy.TokenVersion != 1 {
		t.Fatal("unsafe role migration")
	}
	if err := mongostore.EnsureProductRoles(ctx, db, "wedding"); err != nil {
		t.Fatal(err)
	}
	db.Collection("users").FindOne(ctx, bson.M{"_id": "legacy"}).Decode(&legacy)
	if legacy.TokenVersion != 1 {
		t.Fatal("migration not idempotent")
	}
}
