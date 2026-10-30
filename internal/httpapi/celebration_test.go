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

func TestCelebrationDesignRoundTripAndWhatsAppNotice(t *testing.T) {
	for _, kind := range []string{"wedding", "general"} {
		t.Run(kind, func(t *testing.T) {
			db := testutil.Mongo(t)
			ctx := context.Background()
			u := core.User{ID: "owner", Role: "organizer", Email: "owner@example.com"}
			_, err := db.Collection("users").InsertOne(ctx, u)
			if err != nil {
				t.Fatal(err)
			}
			e := core.Event{ID: "event", Kind: kind, OwnerID: u.ID, PaymentStatus: "unpaid", Slug: "our-day"}
			_, err = db.Collection("events").InsertOne(ctx, e)
			if err != nil {
				t.Fatal(err)
			}
			s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
			token, err := s.sign(u, "password")
			if err != nil {
				t.Fatal(err)
			}
			in := core.DesignInput{Title: "Our celebration", Template: "classic", AccentColor: "#766044", DesignMode: "flyer", Opening: &core.Opening{Enabled: true, Style: "arch", Color: "#F4EBDF", Ink: "#766044", Message: "Open our letter", ButtonLabel: "Abrir"}, Celebration: &core.Celebration{AdultsOnly: true, AdultsMessage: "Solo adultos, con cariño.", WhatsAppNotice: true, DressCode: "Cóctel"}, Sections: []core.Section{{ID: "photo", Icon: "none", Canvas: &core.FlyerCanvas{Width: 720, Height: 960, Background: "#FFFFFF", Elements: []core.FlyerElement{{ID: "image", Type: "image", PhotoKey: "template-photo", Width: 400, Height: 600, Color: "#766044", Font: "serif", FontSize: 40, Align: "center"}}}}}}
			call := func(body core.DesignInput) int {
				raw, _ := json.Marshal(body)
				r := httptest.NewRequest("PATCH", "/events/event/design", strings.NewReader(string(raw)))
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.routes().ServeHTTP(w, r)
				if w.Code != 200 && w.Code != 400 {
					t.Fatalf("unexpected response %d %s", w.Code, w.Body.String())
				}
				return w.Code
			}
			if call(in) != 200 {
				t.Fatal("settings rejected")
			}
			var saved core.Event
			if err = db.Collection("events").FindOne(ctx, bson.M{"_id": "event"}).Decode(&saved); err != nil {
				t.Fatal(err)
			}
			if saved.Opening == nil || saved.Opening.Style != "arch" || saved.Celebration == nil || !saved.Celebration.AdultsOnly {
				t.Fatal("settings lost in Mongo")
			}
			public := publicEventView(saved)
			if public["opening"] == nil || public["celebration"] == nil {
				t.Fatal("settings missing from public invitation")
			}
			raw, _ := json.Marshal(eventMessage(saved, core.Guest{Name: "Ana", Seats: 1, InviteToken: strings.Repeat("a", 48)}))
			if strings.Contains(string(raw), "Solo adultos, con cariño.") {
				t.Fatal("website-only notice leaked into WhatsApp")
			}
			saved.Celebration.WhatsAppNotice = false
			raw, _ = json.Marshal(eventMessage(saved, core.Guest{}))
			if strings.Contains(string(raw), "Solo adultos") {
				t.Fatal("notice sent without checkbox")
			}
			// Old clients must not erase the saved opening when they omit new fields.
			in.Opening = nil
			in.Celebration = nil
			if call(in) != 200 {
				t.Fatal("old client rejected")
			}
			if err = db.Collection("events").FindOne(ctx, bson.M{"_id": "event"}).Decode(&saved); err != nil || saved.Opening == nil {
				t.Fatal("old client erased opening")
			}
			in.Sections[0].Canvas.Elements[0].PhotoKey = "someone-elses.png"
			if call(in) != 400 {
				t.Fatal("foreign photo accepted")
			}
		})
	}
}
