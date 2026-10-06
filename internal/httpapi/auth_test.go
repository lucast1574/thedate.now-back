package httpapi

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/api/idtoken"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionPolicyRejectsStaleAndPasswordAdmin(t *testing.T) {
	t.Setenv("ADMIN_EMAIL", "admin@gmail.com")
	u := core.User{ID: "user", Role: "organizer", Email: "ordinary@gmail.com", TokenVersion: 2}
	for _, claims := range []sessionClaims{{RegisteredClaims: jwt.RegisteredClaims{Subject: u.ID}, Format: 0, Version: 2, Method: "password"}, {RegisteredClaims: jwt.RegisteredClaims{Subject: u.ID}, Format: 1, Version: 1, Method: "password"}, {RegisteredClaims: jwt.RegisteredClaims{Subject: u.ID}, Format: 1, Version: 2, Method: "unknown"}} {
		if validSession(u, &claims) {
			t.Fatal("unsafe session accepted")
		}
	}
	claims := sessionClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: u.ID}, Format: 1, Version: 2, Method: "password"}
	if !validSession(u, &claims) {
		t.Fatal("valid local session rejected")
	}
	u.GoogleSub = "google"
	if validSession(u, &claims) {
		t.Fatal("password session survived Google linking")
	}
	u.Role = "admin"
	u.Email = "admin@gmail.com"
	claims.Method = "google"
	if validSession(u, &claims) {
		t.Fatal("unverified admin accepted")
	}
	u.GoogleAuthoritative = true
	u.IdentityPolicy = 1
	if !validSession(u, &claims) {
		t.Fatal("verified Google admin rejected")
	}
}
func TestGoogleClaimRevokesPreRegistrationAndLegacySessions(t *testing.T) {
	db := testutil.Mongo(t)
	t.Setenv("GOOGLE_CLIENT_ID", "test-client")
	t.Setenv("ADMIN_EMAIL", "victim@gmail.com")
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32)), verifyGoogle: func(context.Context, string, string) (*idtoken.Payload, error) {
		return &idtoken.Payload{Subject: "verified-google", Claims: map[string]any{"email": "victim@gmail.com", "email_verified": true, "name": "Victim"}}, nil
	}}
	u := core.User{ID: "pre-registered", Email: "victim@gmail.com", Role: "planner", PasswordHash: "attacker-password"}
	if _, err := db.Collection("users").InsertOne(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	oldToken, err := s.sign(u, "password")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.googleLogin(w, httptest.NewRequest("POST", "/auth/google", strings.NewReader(`{"idToken":"test","portal":"wedding"}`)))
	if w.Code != 200 {
		t.Fatalf("Google: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		Token string `json:"token"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var linked core.User
	if err = db.Collection("users").FindOne(context.Background(), bson.M{"_id": u.ID}).Decode(&linked); err != nil {
		t.Fatal(err)
	}
	if linked.PasswordHash != "" || linked.TokenVersion != 1 || linked.Role != "admin" {
		t.Fatal("unsafe account linking")
	}
	for _, token := range []string{oldToken, legacyToken(t, s, u.ID)} {
		r := httptest.NewRequest("GET", "/auth/me", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		if _, err = s.user(r); err == nil {
			t.Fatal("old credentials still valid")
		}
	}
	r := httptest.NewRequest("GET", "/auth/me", nil)
	r.Header.Set("Authorization", "Bearer "+result.Token)
	if _, err = s.user(r); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	s.register(w, httptest.NewRequest("POST", "/auth/register", strings.NewReader(`{"email":"victim@gmail.com","password":"abcdefghijk","name":"Attacker","role":"planner"}`)))
	if w.Code != 400 {
		t.Fatal("admin email could register locally")
	}
	w = httptest.NewRecorder()
	s.login(w, httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"email":"victim@gmail.com","password":"attacker-password"}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatal("admin password login accepted")
	}
}
func legacyToken(t *testing.T, s *server, id string) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: id, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}).SignedString(s.secret)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func TestPublicEventOmitsInternalFields(t *testing.T) {
	raw, err := json.Marshal(publicEventView(core.Event{ID: "id", OwnerID: "secret-owner", CoupleUserIDs: []string{"secret-couple"}, CheckoutID: "secret-checkout", PaymentStatus: "paid"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"ownerId", "coupleUserIds", "checkout", "paymentStatus", "responseVersion", "secret-"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("public data leaked %s", secret)
		}
	}
}

func TestNonAuthoritativeGoogleEmailCannotClaimLocalAccount(t *testing.T) {
	db := testutil.Mongo(t)
	t.Setenv("GOOGLE_CLIENT_ID", "test-client")
	t.Setenv("ADMIN_EMAIL", "admin@gmail.com")
	u := core.User{ID: "local", Email: "external@example.test", PasswordHash: "retained-local-hash", Role: "organizer"}
	if _, err := db.Collection("users").InsertOne(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32)), verifyGoogle: func(context.Context, string, string) (*idtoken.Payload, error) {
		return &idtoken.Payload{Subject: "google-external", Claims: map[string]any{"email": u.Email, "email_verified": true}}, nil
	}}
	w := httptest.NewRecorder()
	s.googleLogin(w, httptest.NewRequest("POST", "/auth/google", strings.NewReader(`{"idToken":"test","portal":"general"}`)))
	if w.Code != 409 {
		t.Fatalf("non-authoritative linking: %d", w.Code)
	}
	var stored core.User
	if err := db.Collection("users").FindOne(context.Background(), bson.M{"_id": u.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.PasswordHash != u.PasswordHash || stored.GoogleSub != "" || stored.TokenVersion != 0 {
		t.Fatal("local credentials changed without sufficient ownership proof")
	}
}
