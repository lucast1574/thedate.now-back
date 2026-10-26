package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
)

func supportFixture(t *testing.T) (*server, map[string]string) {
	t.Helper()
	t.Setenv("ADMIN_EMAIL", "root@example.test")
	db := testutil.Mongo(t)
	if err := mongostore.EnsureIndexes(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	s := &server{product: "general", db: db, secret: productSecret([]byte("test-secret"), "general")}
	tokens := map[string]string{}
	for _, role := range []string{"owner", "other", "admin"} {
		u := core.User{ID: role, Name: "Test " + role, Email: role + "@example.test", Role: "organizer", Product: "general"}
		if role == "admin" {
			u.Role = "admin"
		}
		if _, err := db.Collection("users").InsertOne(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		token, err := s.sign(u, "password")
		if err != nil {
			t.Fatal(err)
		}
		tokens[role] = token
	}
	return s, tokens
}
func supportCall(s *server, token, method, path string, input any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(input)
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	return w
}
func screenshotFixture(t *testing.T) string {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
}
func supportDraft(t *testing.T) supportInput {
	return supportInput{RequestID: uuid.NewString(), Title: "No puedo guardar mi invitación", Description: "Los cambios del editor no aparecen después de guardar.", Images: []string{screenshotFixture(t)}}
}
func TestSupportOwnershipAttachmentsAndAdminAccess(t *testing.T) {
	s, tokens := supportFixture(t)
	in := supportDraft(t)
	if w := supportCall(s, "", "POST", "/support/tickets", in); w.Code != 401 {
		t.Fatal("anonymous creation allowed")
	}
	w := supportCall(s, tokens["owner"], "POST", "/support/tickets", in)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var ticket supportView
	if err := json.Unmarshal(w.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	if ticket.Requester != nil || len(ticket.Screenshots) != 1 || strings.Contains(w.Body.String(), "requestId") || strings.Contains(w.Body.String(), "requesterEmail") || strings.Contains(w.Body.String(), "actorId") {
		t.Fatal("private storage fields leaked")
	}
	for _, entry := range []struct {
		role, path string
		want       int64
	}{{"owner", "/support/tickets", 1}, {"other", "/support/tickets", 0}, {"admin", "/admin/support/tickets?state=OPEN&page=1", 1}} {
		w = supportCall(s, tokens[entry.role], "GET", entry.path, nil)
		var list struct {
			Total int64         `json:"total"`
			Items []supportView `json:"items"`
		}
		if json.Unmarshal(w.Body.Bytes(), &list) != nil || w.Code != 200 || list.Total != entry.want {
			t.Fatalf("list %s: %d %s", entry.role, w.Code, w.Body.String())
		}
		if entry.role == "admin" && (list.Items[0].Requester == nil || list.Items[0].Requester.Email != "owner@example.test") {
			t.Fatal("admin requester missing")
		}
	}
	if supportCall(s, tokens["other"], "GET", "/admin/support/tickets", nil).Code != 403 {
		t.Fatal("ordinary user read admin inbox")
	}
	for _, role := range []string{"owner", "admin", "other"} {
		w = supportCall(s, tokens[role], "GET", "/support/tickets/"+ticket.ID+"/images/0", nil)
		if role == "other" {
			if w.Code != 404 {
				t.Fatal("another user read screenshot")
			}
			continue
		}
		if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("screenshot not private")
		}
	}
	if supportCall(s, tokens["owner"], "PATCH", "/admin/support/tickets/"+ticket.ID, map[string]any{"state": "DONE", "version": 0}).Code != 403 {
		t.Fatal("requester changed status")
	}
	if supportCall(s, tokens["owner"], "GET", "/support/tickets?state=invalid", nil).Code != 400 {
		t.Fatal("invalid filter accepted")
	}
	if supportCall(s, tokens["admin"], "GET", "/admin/support/tickets?page=100001", nil).Code != 400 {
		t.Fatal("unbounded pagination")
	}
	foreign := &server{product: "wedding", db: testutil.Mongo(t), secret: productSecret([]byte("test-secret"), "wedding")}
	if supportCall(foreign, tokens["owner"], "GET", "/support/tickets", nil).Code != 401 {
		t.Fatal("cross-brand token accepted")
	}
	if _, err := (mongostore.SupportTickets{DB: foreign.db}).Get(context.Background(), ticket.ID, "", false); err == nil {
		t.Fatal("ticket crossed product databases")
	}
}
func TestSupportInvalidImagesAndOversizeRequests(t *testing.T) {
	for _, images := range [][]string{{"data:image/svg+xml;base64,PHN2Zz4="}, {"data:image/png;base64,bm90LWFuLWltYWdl"}, {strings.Repeat("x", 1500000)}} {
		if _, err := supportImages(images); err == nil {
			t.Fatal("unsafe image accepted")
		}
	}
	valid := screenshotFixture(t)
	if _, err := supportImages([]string{valid, valid, valid, valid}); err == nil {
		t.Fatal("too many screenshots accepted")
	}
	s, tokens := supportFixture(t)
	in := supportDraft(t)
	in.Title = "x"
	if supportCall(s, tokens["owner"], "POST", "/support/tickets", in).Code != 400 {
		t.Fatal("invalid ticket accepted")
	}
	in = supportDraft(t)
	in.Images = []string{valid, valid, valid, valid}
	if supportCall(s, tokens["owner"], "POST", "/support/tickets", in).Code != 400 {
		t.Fatal("invalid image submission accepted")
	}
}
