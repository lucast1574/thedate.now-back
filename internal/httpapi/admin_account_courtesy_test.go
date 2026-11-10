package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestAccountCourtesyFundsExactlyOneInvitation(t *testing.T) {
	for _, kind := range []string{"general", "wedding"} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.Mongo(t)
			ctx := context.Background()
			s := &server{db: db, product: kind, secret: []byte(strings.Repeat("c", 32))}
			if err := mongostore.EnsureIndexes(ctx, db); err != nil {
				t.Fatal(err)
			}
			admin := core.User{ID: "admin", Email: "admin@example.test", Role: "admin"}
			user := core.User{ID: "tester", Email: "tester@example.test", Role: "user"}
			for _, u := range []core.User{admin, user} {
				if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.ensureDemos(ctx, user); err != nil {
				t.Fatal(err)
			}
			id := "demo-" + kind + "-" + user.ID
			request := func(u core.User, method, path string, input any) *httptest.ResponseRecorder {
				body, _ := json.Marshal(input)
				token, err := s.sign(u, "password")
				if err != nil {
					t.Fatal(err)
				}
				r := httptest.NewRequest(method, path, strings.NewReader(string(body)))
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.routes().ServeHTTP(w, r)
				return w
			}
			path := "/admin/users/" + user.ID + "/events"
			if w := request(user, "GET", path, nil); w.Code != 403 {
				t.Fatal("nonadmin listed events", w.Code)
			}
			if w := request(admin, "GET", path, nil); w.Code != 200 || !strings.Contains(w.Body.String(), id) {
				t.Fatal("missing demo", w.Code, w.Body.String())
			}
			if w := request(admin, "GET", "/admin/users?email=%20TESTER%40EXAMPLE.TEST%20", nil); w.Code != 200 || !strings.Contains(w.Body.String(), user.ID) {
				t.Fatal("email lookup failed", w.Body.String())
			}
			grant := func(u core.User) *httptest.ResponseRecorder {
				return request(u, "POST", "/admin/events/"+id+"/courtesy", map[string]string{"reason": "Tester invitation"})
			}
			if w := grant(user); w.Code != 403 {
				t.Fatal("nonadmin granted courtesy")
			}
			if w := grant(admin); w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			if w := grant(admin); w.Code != 409 {
				t.Fatal("courtesy duplicated")
			}
			input := func(slug string) core.EventInput {
				return core.EventInput{Kind: kind, Slug: slug, Title: "Celebration", StartAt: time.Now().Add(48 * time.Hour), TimeZone: "America/Lima", Organizer: "Tester", Location: "Lima", MapURL: "https://maps.google.com/", CapacityUnlimited: true, Template: "classic", AccentColor: "#996644"}
			}
			var group sync.WaitGroup
			results := make(chan *httptest.ResponseRecorder, 2)
			for _, slug := range []string{"tester-first", "tester-second"} {
				group.Add(1)
				go func(slug string) { defer group.Done(); results <- request(user, "POST", "/events", input(slug)) }(slug)
			}
			group.Wait()
			close(results)
			paid := 0
			for w := range results {
				if w.Code != 201 && w.Code != 409 {
					t.Fatal(w.Code, w.Body.String())
				}
				if w.Code == 201 {
					var event core.Event
					json.Unmarshal(w.Body.Bytes(), &event)
					if event.PaymentStatus == "paid" {
						paid++
						if event.IsDemo || event.PaymentSource != "courtesy" || event.Payment != nil || event.ID != id {
							t.Fatal("invalid courtesy", event)
						}
					}
				}
			}
			if paid != 1 {
				t.Fatal("expected exactly one free invitation", paid)
			}
			count, err := db.Collection("events").CountDocuments(ctx, bson.M{"ownerId": user.ID, "paymentStatus": "paid"})
			if err != nil || count != 1 {
				t.Fatal("courtesy spent twice", count, err)
			}
			if w := grant(admin); w.Code != 409 {
				t.Fatal("consumed courtesy granted again")
			}
			if err := s.ensureDemos(ctx, user); err != nil {
				t.Fatal(err)
			}
			w := request(user, "POST", "/events", input("tester-third"))
			var third core.Event
			json.Unmarshal(w.Body.Bytes(), &third)
			if w.Code != 201 || third.PaymentStatus != "unpaid" {
				t.Fatal("following invitation was free", w.Code, w.Body.String())
			}
		})
	}
}
