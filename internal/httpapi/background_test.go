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

func TestBackgroundRoundTripOwnershipAndLegacyUpdates(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	u := core.User{ID: "owner", Role: "planner", Email: "qa@example.test"}
	db.Collection("users").InsertOne(ctx, u)
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	token, err := s.sign(u, "password")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"wedding", "general"} {
		e := core.Event{ID: kind, Kind: kind, OwnerID: u.ID, PhotoKeys: []string{"mine.jpg"}}
		if _, err = db.Collection("events").InsertOne(ctx, e); err != nil {
			t.Fatal(err)
		}
		bg := &core.ImageBackground{PhotoKey: "mine.jpg", Fit: "cover", Position: "top", Opacity: .5}
		in := core.DesignInput{Title: "Our day", Template: "classic", DesignMode: "flyer", AccentColor: "#766044", BackgroundImage: bg, Sections: []core.Section{{ID: "one", Icon: "none", Canvas: &core.FlyerCanvas{Width: 720, Height: 960, Background: "#FFFFFF", BackgroundImage: bg, Elements: []core.FlyerElement{}}}}}
		in.Opening = &core.Opening{Enabled: true, Style: "classic", Color: "#FFFFFF", Ink: "#766044", ButtonLabel: "Open", BackgroundImage: bg}

		call := func(body core.DesignInput) int {
			raw, _ := json.Marshal(body)
			r := httptest.NewRequest("PATCH", "/events/"+e.ID+"/design", strings.NewReader(string(raw)))
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.routes().ServeHTTP(w, r)
			return w.Code
		}
		if call(in) != 200 {
			t.Fatal("background rejected")
		}
		var saved core.Event
		load := func() {
			if err = db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved); err != nil {
				t.Fatal(err)
			}
		}
		load()
		if saved.Opening == nil || saved.Opening.BackgroundImage == nil || saved.BackgroundImage == nil || saved.BackgroundImage.PhotoKey != "mine.jpg" || saved.Sections[0].Canvas.BackgroundImage.Position != "top" || publicEventView(saved)["backgroundImage"] == nil {
			t.Fatal("backgrounds lost")
		}
		in.BackgroundImage = nil
		if call(in) != 200 {
			t.Fatal("legacy rejected")
		}
		load()
		if saved.BackgroundImage == nil {
			t.Fatal("legacy erased background")
		}
		in.ClearBackgroundImage = true
		if call(in) != 200 {
			t.Fatal("clear rejected")
		}
		load()
		if saved.BackgroundImage != nil {
			t.Fatal("clear ignored")
		}
		in.BackgroundImage = &core.ImageBackground{PhotoKey: "foreign.jpg", Fit: "cover", Position: "center", Opacity: 1}
		if call(in) != 400 {
			t.Fatal("foreign common background accepted")
		}
		in.BackgroundImage = nil
		in.Sections[0].Canvas.BackgroundImage = nil
		in.Sections[0].Canvas.BackgroundImage = &core.ImageBackground{PhotoKey: "foreign.jpg", Fit: "cover", Position: "center", Opacity: 1}
		if call(in) != 400 {
			t.Fatal("foreign section background accepted")
		}
		in.Sections[0].Canvas.BackgroundImage = nil
		in.Opening.BackgroundImage = &core.ImageBackground{PhotoKey: "foreign.jpg", Fit: "cover", Position: "center", Opacity: 1}
		if call(in) != 400 {
			t.Fatal("foreign opening background accepted")
		}

	}
}
