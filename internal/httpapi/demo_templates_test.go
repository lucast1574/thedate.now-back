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
)

func TestDemoAcceptsEveryOwnTemplateAndGiftElement(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	u := core.User{ID: "owner", Role: "planner", Email: "qa@example.test"}
	if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, err := s.sign(u, "password")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"wedding", "general"} {
		e := core.Event{ID: "demo-" + kind + "-owner", Kind: kind, OwnerID: u.ID, IsDemo: true, PaymentStatus: "demo"}
		if _, err = db.Collection("events").InsertOne(ctx, e); err != nil {
			t.Fatal(err)
		}
		for _, template := range core.TemplateCatalog() {
			if template.Mode != "flyer" || template.Kind != kind {
				continue
			}
			in := core.DesignInput{Title: "Our demo", Template: template.Style, TemplateID: template.ID, DesignMode: "flyer", AccentColor: template.Accent, Opening: &core.Opening{Enabled: true, Style: "classic", Color: template.Background, Ink: template.Accent, Title: "Our letter", Font: "sans", SealIcon: "none", ButtonLabel: "Open"}}
			in.Sections = []core.Section{{ID: "gifts", Icon: "none", Canvas: &core.FlyerCanvas{Width: 720, Height: 960, Background: template.Background, Elements: []core.FlyerElement{
				{ID: "link", Type: "text", Binding: "gift_link", Link: "/regalos", Text: "Our gifts", Width: 400, Height: 120, Font: template.Font, FontSize: 30, Color: template.Accent, Align: "center"},
				{ID: "dress", Type: "text", Binding: "dress_code", DressStyle: "all", DressIcon: "lucide:shirt", DressPhotoKey: "template-photo", Text: "Formal", Y: 200, Width: 400, Height: 360, Font: template.Font, FontSize: 30, Color: template.Accent, Align: "center"},
			}}}}
			raw, _ := json.Marshal(in)
			r := httptest.NewRequest("PATCH", "/events/"+e.ID+"/design", strings.NewReader(string(raw)))
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.routes().ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatalf("%s demo save: %d %s", template.ID, w.Code, w.Body.String())
			}
			var saved core.Event
			if err = db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved); err != nil {
				t.Fatal(err)
			}
			if !saved.IsDemo || saved.PaymentStatus != "demo" || saved.TemplateID != template.ID || saved.Opening.Title != "Our letter" || saved.Opening.SealIcon != "none" {
				t.Fatal("demo template or opening not persisted")
			}
			items := saved.Sections[0].Canvas.Elements
			if items[0].Binding != "gift_link" || items[0].Link != "/regalos" || items[1].DressStyle != "all" || items[1].DressPhotoKey != "template-photo" {
				t.Fatal("rich elements lost")
			}
		}
	}
}
