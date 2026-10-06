package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFreeAdminEventsCannotBeRequestedByLocalAccounts(t *testing.T) {
	db := testutil.Mongo(t)
	s := &server{db: db, secret: []byte(strings.Repeat("s", 32))}
	admin := core.User{ID: "admin", Email: "admin@gmail.com", Role: "admin", GoogleSub: "google", GoogleAuthoritative: true, IdentityPolicy: 1}
	member := core.User{ID: "member", Role: "couple", Email: "member@example.test"}
	for _, u := range []core.User{admin, member} {
		_, _ = db.Collection("users").InsertOne(context.Background(), u)
	}
	for _, kind := range []string{"general", "wedding"} {
		for _, u := range []core.User{admin, member} {
			input := core.EventInput{Kind: kind, Slug: kind + "-" + u.ID, Title: "Celebration", StartAt: time.Now().Add(time.Hour), TimeZone: "America/Lima", Organizer: "Lucas", Location: "Lima", MapURL: "https://maps.google.com/", CapacityUnlimited: true, Template: "classic", AccentColor: "#996644"}
			body, _ := json.Marshal(input)
			method := "password"
			if u.Role == "admin" {
				method = "google"
			}
			token, _ := s.sign(u, method)
			r := httptest.NewRequest("POST", "/events", strings.NewReader(string(body)))
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.createEvent(w, r)
			if w.Code != 201 {
				t.Fatalf("create %s %s: %d %s", u.Role, kind, w.Code, w.Body.String())
			}
			var e core.Event
			_ = json.Unmarshal(w.Body.Bytes(), &e)
			if u.Role == "admin" && (e.PaymentStatus != "paid" || e.PaymentSource != "courtesy") {
				t.Fatal("admin charged")
			}
			if u.Role != "admin" && (e.PaymentStatus != "unpaid" || e.PaymentSource != "") {
				t.Fatal("nonadmin got courtesy")
			}
			injected := strings.TrimSuffix(string(body), "}") + `,"paymentStatus":"paid","paymentSource":"courtesy","role":"admin"}`
			r = httptest.NewRequest("POST", "/events", strings.NewReader(injected))
			r.Header.Set("Authorization", "Bearer "+token)
			w = httptest.NewRecorder()
			s.createEvent(w, r)
			if w.Code != 400 {
				t.Fatal("client forged paid status")
			}
		}
	}
}
