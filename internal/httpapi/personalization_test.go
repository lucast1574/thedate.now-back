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

func TestSentInvitationKeepsItsLinkAndReadsLatestDesign(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	owner := core.User{ID: "owner", Email: "owner@example.test", Role: "planner"}
	db.Collection("users").InsertOne(ctx, owner)
	e := core.Event{ID: "event", OwnerID: owner.ID, Kind: "wedding", Slug: "our-day", Title: "Our wedding", Template: "classic", PaymentStatus: "paid", PublishedAt: &now}
	db.Collection("events").InsertOne(ctx, e)
	g := core.Guest{ID: "guest", EventID: e.ID, Name: "Ana", LastName: "Rojas", Phone: "+51987654321", InviteToken: strings.Repeat("a", 48), Seats: 2, SentAt: &now, InvitationMessageID: "sent-message"}
	db.Collection("guests").InsertOne(ctx, g)
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, _ := s.sign(owner, "password")
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w
	}
	before := request("GET", "/public/rsvp/"+g.InviteToken, "")
	if before.Code != 200 || !strings.Contains(before.Body.String(), `"guestName":"Ana Rojas"`) {
		t.Fatal("personal name missing")
	}
	payload := `{"title":"A new title","template":"classic","accentColor":"#995d72","sections":[{"id":"personal","icon":"none","heading":"For you","body":"","guestText":{"id":"name","type":"text","binding":"guest_name","text":"Hola, {{nombre_invitado}}","x":0,"y":40,"width":720,"height":160,"rotation":15,"color":"#995d72","font":"script","fontSize":48,"align":"center"}}]}`
	if w := request("PATCH", "/events/event/design", payload); w.Code != 200 {
		t.Fatalf("published save rejected: %s", w.Body.String())
	}
	after := request("GET", "/public/rsvp/"+g.InviteToken, "")
	var view map[string]any
	json.Unmarshal(after.Body.Bytes(), &view)
	event := view["event"].(map[string]any)
	if event["title"] != "A new title" || event["slug"] != e.Slug || event["ownerId"] != nil || event["paymentStatus"] != nil {
		t.Fatal("live design or allowlist incorrect")
	}
	var saved core.Guest
	db.Collection("guests").FindOne(ctx, bson.M{"_id": g.ID}).Decode(&saved)
	if saved.InviteToken != g.InviteToken || saved.InvitationMessageID != g.InvitationMessageID || saved.SentAt == nil {
		t.Fatal("design update changed recipient or sent message")
	}
	db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"paymentStatus": "unpaid"}})
	if w := request("GET", "/public/rsvp/"+g.InviteToken, ""); w.Code != 404 {
		t.Fatal("unpaid personalized invitation leaked")
	}
}
