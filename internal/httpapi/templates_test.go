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
	"testing"
)

func TestTemplateCatalogAndSavedDesignKeepProductIdentity(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	u := core.User{ID: "owner", Email: "test@example.test", Role: "planner"}
	db.Collection("users").InsertOne(ctx, u)
	e := core.Event{ID: "e", Kind: "wedding", OwnerID: u.ID, Template: "classic"}
	db.Collection("events").InsertOne(ctx, e)
	if err := mongostore.EnsureTemplates(ctx, db); err != nil {
		t.Fatal(err)
	}
	var legacy core.Event
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&legacy); err != nil || legacy.TemplateID != "wedding-classic" {
		t.Fatal("legacy product not backfilled")
	}
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, err := s.sign(u, "password")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w
	}
	w := call("GET", "/templates", "")
	var catalog []core.InvitationTemplate
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &catalog) != nil || len(catalog) != 12 {
		t.Fatal("catalog not loaded from database")
	}
	payload := `{"title":"Our wedding","template":"classic","templateId":"general-classic","accentColor":"#9c6449","designMode":"sections","sections":[]}`
	if w = call("PATCH", "/events/e/design", payload); w.Code != 400 {
		t.Fatal("cross-product design accepted")
	}
	payload = strings.ReplaceAll(payload, "general-classic", "wedding-sections-romance")
	if w = call("PATCH", "/events/e/design", payload); w.Code != 200 {
		t.Fatalf("save: %s", w.Body.String())
	}
	var saved core.Event
	if err = db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved); err != nil || saved.TemplateID != "wedding-sections-romance" {
		t.Fatal("template selection not persisted")
	}
	if err = mongostore.EnsureTemplates(ctx, db); err != nil {
		t.Fatal(err)
	}
	db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved)
	if saved.TemplateID != "wedding-sections-romance" {
		t.Fatal("catalog initialization replaced saved template")
	}
	if publicEventView(saved)["templateId"] != saved.TemplateID {
		t.Fatal("public identity lost")
	}
}
