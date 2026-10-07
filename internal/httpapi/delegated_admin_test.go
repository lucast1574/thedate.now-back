package httpapi

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testLogin struct {
	User  core.User `json:"user"`
	Token string    `json:"token"`
}

func routeRequest(t *testing.T, s *server, token, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	return w
}
func decodeLogin(t *testing.T, w *httptest.ResponseRecorder) testLogin {
	t.Helper()
	var result testLogin
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.User.ID == "" || result.Token == "" {
		t.Fatal("missing account or session")
	}
	return result
}
func TestDelegatedPasswordAdminLoginRevocationAndEscalationProtection(t *testing.T) {
	db := testutil.Mongo(t)
	t.Setenv("ADMIN_EMAIL", "owner@gmail.com")
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	root := core.User{ID: "owner", Email: "owner@gmail.com", Role: "admin", GoogleSub: "root", GoogleAuthoritative: true, IdentityPolicy: 1}
	if _, err := db.Collection("users").InsertOne(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	rootToken, err := s.sign(root, "google")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"email":"delegate@example.test","password":"test-password-123","name":"Delegate","role":"organizer"}`
	ordinary := decodeLogin(t, routeRequest(t, s, "", "POST", "/auth/register", body, 201))
	path := "/admin/users/" + ordinary.User.ID + "/role"
	routeRequest(t, s, ordinary.Token, "PATCH", path, `{"role":"admin"}`, 403)
	routeRequest(t, s, "", "PATCH", path, `{"role":"admin"}`, 401)
	routeRequest(t, s, "", "POST", "/auth/register", `{"email":"fake@example.test","password":"test-password-123","name":"Fake","role":"admin"}`, 400)
	routeRequest(t, s, rootToken, "PATCH", path, `{"role":"invalid"}`, 400)
	routeRequest(t, s, rootToken, "PATCH", path, `{"role":"admin"}`, 200)
	routeRequest(t, s, ordinary.Token, "GET", "/auth/me", "", 401)
	routeRequest(t, s, "", "POST", "/auth/login", `{"email":"delegate@example.test","password":"wrong-password"}`, 401)
	delegated := decodeLogin(t, routeRequest(t, s, "", "POST", "/auth/login", body, 200))
	if delegated.User.Role != "admin" {
		t.Fatal("delegated role was not persisted")
	}
	routeRequest(t, s, delegated.Token, "GET", "/admin/overview", "", 200)
	routeRequest(t, s, delegated.Token, "PATCH", path, `{"role":"organizer"}`, 403)
	routeRequest(t, s, delegated.Token, "PATCH", "/admin/users/owner/role", `{"role":"organizer"}`, 403)
	routeRequest(t, s, delegated.Token, "PATCH", "/profile", `{"role":"admin"}`, 400)
	routeRequest(t, s, rootToken, "PATCH", path, `{"role":"planner"}`, 200)
	routeRequest(t, s, delegated.Token, "GET", "/admin/overview", "", 401)
	demoted := decodeLogin(t, routeRequest(t, s, "", "POST", "/auth/login", body, 200))
	routeRequest(t, s, demoted.Token, "GET", "/admin/overview", "", 403)
	claims := jwt.MapClaims{"sub": ordinary.User.ID, "iss": "thedate-api", "aud": "thedate-studio", "exp": time.Now().Add(time.Hour).Unix(), "format": 1, "version": 2, "method": "password", "role": "admin"}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("attacker-controlled-secret"))
	if err != nil {
		t.Fatal(err)
	}
	routeRequest(t, s, forged, "GET", "/admin/overview", "", 401)
}
