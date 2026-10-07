package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAvatarRejectsInvalidAndOversizedImages(t *testing.T) {
	if _, err := avatarImage([]byte(`<svg onload="alert(1)"></svg>`)); err == nil {
		t.Fatal("SVG accepted")
	}
	var raw bytes.Buffer
	_ = png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 513, 1)))
	if _, err := avatarImage(raw.Bytes()); err == nil {
		t.Fatal("oversized image accepted")
	}
}

func TestOwnProfileAndAdminDemoIsolation(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	users := []core.User{{ID: "root", Email: "root@gmail.com", Name: "Root", Role: "admin", GoogleSub: "root", GoogleAuthoritative: true, IdentityPolicy: 1}, {ID: "other", Email: "other@example.test", Name: "Other", Role: "organizer"}}
	for _, u := range users {
		if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ensureDemos(ctx, users[1]); err != nil {
		t.Fatal(err)
	}
	token, _ := s.sign(users[0], "google")
	otherToken, _ := s.sign(users[1], "password")
	call := func(method, path, body string, auth string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+auth)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w
	}
	w := call("GET", "/events", "", token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var events []core.Event
	_ = json.Unmarshal(w.Body.Bytes(), &events)
	if len(events) != 2 {
		t.Fatal("admin sees somebody else's demo", len(events))
	}
	for _, e := range events {
		if e.OwnerID != "root" {
			t.Fatal("wrong demo owner")
		}
	}
	if w = call("PATCH", "/profile", `{"name":"New Name","role":"admin"}`, otherToken); w.Code != 400 {
		t.Fatal("privilege field accepted", w.Code)
	}
	if w = call("PATCH", "/profile", `{"name":"New Name"}`, otherToken); w.Code != 200 {
		t.Fatal(w.Code)
	}
	var stored core.User
	_ = db.Collection("users").FindOne(ctx, bson.M{"_id": "other"}).Decode(&stored)
	if stored.Role != "organizer" || stored.Name != "New Name" {
		t.Fatal("profile changed permissions")
	}
	var photo bytes.Buffer
	_ = png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 32, 32)))
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, _ := writer.CreateFormFile("photo", "avatar.png")
	_, _ = part.Write(photo.Bytes())
	_ = writer.Close()
	r := httptest.NewRequest("POST", "/profile/avatar", &form)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+otherToken)
	w = httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("upload", w.Code, w.Body.String())
	}
	if w = call("GET", "/profile/avatar", "", token); w.Code != 404 {
		t.Fatal("another user read photo")
	}
	if w = call("GET", "/profile/avatar", "", otherToken); w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal("own photo missing")
	}
	if w = call("GET", "/profile/avatar", "", ""); w.Code != 401 {
		t.Fatal("anonymous access")
	}
}
