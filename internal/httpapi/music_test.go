package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMusicDesignOwnsAudioAndCanRemoveSong(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	u := core.User{ID: "music-owner", Email: "music-owner@example.com", Role: "organizer"}
	if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
		t.Fatal(err)
	}
	key := "b6ea72ce-c49d-4f94-ae71-0a69d23dbf1e.mp3"
	e := core.Event{ID: "music-event", Kind: "wedding", OwnerID: u.ID, AudioKeys: []string{key}}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, secret: []byte(strings.Repeat("m", 32))}
	token, err := s.sign(u, "password")
	if err != nil {
		t.Fatal(err)
	}
	save := func(music string) int {
		body := `{"title":"Our wedding","description":"","template":"classic","accentColor":"#826148","sections":[],"music":` + music + `}`
		r := httptest.NewRequest("PATCH", "/events/music-event/design", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w.Code
	}
	if status := save(`{"source":"mp3","value":"foreign.mp3","title":"Other song"}`); status != 400 {
		t.Fatalf("foreign audio accepted: %d", status)
	}
	if status := save(`{"source":"mp3","value":"` + key + `","title":"Our song"}`); status != 200 {
		t.Fatalf("owned song: %d", status)
	}
	var stored core.Event
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Music == nil || stored.Music.Value != key {
		t.Fatal("music was not persisted")
	}
	view := publicEventView(stored)
	if view["music"] == nil {
		t.Fatal("public view lost song")
	}
	if _, exists := view["audioKeys"]; exists {
		t.Fatal("public view leaks draft audio library")
	}
	if status := save("null"); status != 200 {
		t.Fatalf("remove music: %d", status)
	}
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Music != nil {
		t.Fatal("removed music still present")
	}
}
