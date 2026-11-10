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
	"time"
)

func TestCreateBlankInvitationAndEditCanvas(t *testing.T) {
	for _, kind := range []string{"general", "wedding"} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.Mongo(t)
			ctx := context.Background()
			s := &server{db: db, product: kind, secret: []byte(strings.Repeat("b", 32))}
			u := core.User{ID: "creator", Role: "user", Email: "creator@example.test"}
			if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
				t.Fatal(err)
			}
			token, _ := s.sign(u, "password")
			call := func(method, path string, input any) *httptest.ResponseRecorder {
				body, _ := json.Marshal(input)
				r := httptest.NewRequest(method, path, strings.NewReader(string(body)))
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.routes().ServeHTTP(w, r)
				return w
			}
			design := core.DesignInput{Title: "Mi celebración", Template: "classic", TemplateID: kind + "-blank", DesignMode: "flyer", AccentColor: "#7451ac", Sections: []core.Section{{ID: "blank-canvas", Icon: "none", Heading: "Sección 1", Canvas: &core.FlyerCanvas{Width: 720, Height: 960, Background: "#ffffff", Elements: []core.FlyerElement{}}}}}
			input := core.EventInput{Kind: kind, Slug: "desde-cero", Title: design.Title, Template: design.Template, TemplateID: design.TemplateID, DesignMode: design.DesignMode, AccentColor: design.AccentColor, StartAt: time.Now().Add(48 * time.Hour), TimeZone: "America/Lima", Organizer: "Lucas", Location: "Lima", MapURL: "https://maps.google.com/", CapacityUnlimited: true, InitialDesign: &design}
			w := call("POST", "/events", input)
			if w.Code != 201 {
				t.Fatal(w.Code, w.Body.String())
			}
			var event core.Event
			if json.Unmarshal(w.Body.Bytes(), &event) != nil {
				t.Fatal("invalid event")
			}
			if event.TemplateID != kind+"-blank" || len(event.Sections) != 1 || len(event.Sections[0].Canvas.Elements) != 0 || event.Opening != nil || event.PaymentStatus != "unpaid" {
				t.Fatal("blank invitation was not blank", event)
			}
			if err := mongostore.EnsureTemplates(ctx, db); err != nil {
				t.Fatal(err)
			}
			var saved core.Event
			db.Collection("events").FindOne(ctx, bson.M{"_id": event.ID}).Decode(&saved)
			if saved.TemplateID != kind+"-blank" {
				t.Fatal("catalog replaced blank identity")
			}
			design.Sections[0].Canvas.Background = "#fff9ed"
			if w = call("PATCH", "/events/"+event.ID+"/design", design); w.Code != 200 {
				t.Fatal("blank canvas cannot be edited", w.Code, w.Body.String())
			}
			for _, template := range core.TemplateCatalog() {
				if template.Kind == kind && template.Mode == "flyer" {
					design.TemplateID = template.ID
					design.Template = template.Style
					break
				}
			}
			if w = call("PATCH", "/events/"+event.ID+"/design", design); w.Code != 200 {
				t.Fatal("template cannot be chosen later", w.Code, w.Body.String())
			}
		})
	}
}
