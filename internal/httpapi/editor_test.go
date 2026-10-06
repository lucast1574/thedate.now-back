package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnpaidDesignSavesButGuestToolsRequirePayment(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	u := core.User{ID: "owner", Email: "owner@example.com", Role: "organizer"}
	if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
		t.Fatal(err)
	}
	e := core.Event{ID: "free", Kind: "general", OwnerID: u.ID, PaymentStatus: "unpaid", PhotoKeys: []string{"ours.jpg"}}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, err := s.sign(u, "password")
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"title":"My invitation","description":"Free draft","template":"classic","accentColor":"#7451ac","designMode":"flyer","sections":[{"id":"cover","icon":"heart","heading":"Cover","body":"","canvas":{"width":720,"height":960,"background":"#fff9ed","elements":[{"id":"photo","type":"image","x":10,"y":10,"width":400,"height":300,"rotation":20,"photoKey":"ours.jpg","color":"#7451ac","font":"serif","fontSize":40,"bold":false,"align":"center"}]}}]}`
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w
	}
	w := call("PATCH", "/events/free/design", payload)
	if w.Code != 200 {
		t.Fatalf("free design %d: %s", w.Code, w.Body.String())
	}
	var saved core.Event
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": "free"}).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(publicEventView(saved))
	if !strings.Contains(string(raw), `"designMode":"flyer"`) || saved.Sections[0].Canvas.Elements[0].Rotation != 20 {
		t.Fatal("canvas did not round-trip")
	}
	for _, test := range []struct{ path, body string }{{"/events/free/guests", `{"name":"Ana","phone":"+51999999999","seats":1}`}, {"/events/free/guests/import", `{"guests":[{"name":"Ana","phone":"+51999999999","seats":1}]}`}, {"/events/free/send-invitations", `{}`}} {
		if response := call(http.MethodPost, test.path, test.body); response.Code != 403 {
			t.Fatalf("unpaid tool %s allowed: %d", test.path, response.Code)
		}
	}
	if _, err := db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"paymentStatus": "paid"}}); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		w = call("POST", "/events/free/guests/import", `{"guests":[{"name":"Ana","phone":"+51999999999","seats":1},{"name":"Ana again","phone":"+51999999999","seats":1}]}`)
		if w.Code != 200 {
			t.Fatalf("import: %s", w.Body.String())
		}
	}
	count, err := db.Collection("guests").CountDocuments(ctx, bson.M{"eventId": e.ID})
	if err != nil || count != 1 {
		t.Fatalf("duplicate import guests: %d %v", count, err)
	}
}
func TestWazendBrandsAndDedicatedSessions(t *testing.T) {
	t.Setenv("WAZEND_SESSION", "shared")
	t.Setenv("WAZEND_WEDDING_SESSION", "weddings")
	t.Setenv("WAZEND_GENERAL_SESSION", "events")
	if wazendFor("wedding").Session != "weddings" || wazendFor("general").Session != "events" {
		t.Fatal("wrong sessions")
	}
	for _, kind := range []string{"wedding", "general"} {
		e := core.Event{Kind: kind, Slug: "our-day", Title: "A day"}
		g := core.Guest{Name: "Ana"}
		raw, _ := json.Marshal(eventMessage(e, g))
		if !strings.Contains(string(raw), invitationBrand(kind)) {
			t.Fatal("missing brand")
		}
	}
}
