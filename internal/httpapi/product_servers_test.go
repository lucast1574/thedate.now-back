package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"google.golang.org/api/idtoken"
)

func TestProductSessionsCannotCrossBrands(t *testing.T) {
	master := []byte(strings.Repeat("x", 32))
	general := &server{product: "general", secret: productSecret(master, "general")}
	wedding := &server{product: "wedding", secret: productSecret(master, "wedding")}
	if bytes.Equal(general.secret, wedding.secret) {
		t.Fatal("shared session key")
	}
	token, err := general.sign(core.User{ID: "one", Product: "general", Role: "organizer"}, "password")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/auth/me", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	if _, err := wedding.user(r); err == nil {
		t.Fatal("foreign brand session accepted")
	}
	w := httptest.NewRecorder()
	productRoutes([]*server{general, wedding}).ServeHTTP(w, httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{}`)))
	if w.Code != 404 {
		t.Fatal("legacy shared authentication is still reachable")
	}
	w = httptest.NewRecorder()
	wedding.googleLogin(w, httptest.NewRequest("POST", "/auth/google", strings.NewReader(`{"portal":"general","idToken":"foreign"}`)))
	if w.Code != 400 {
		t.Fatal("foreign Google portal accepted")
	}
}

func TestIsolatedStartupRejectsSharedConfiguration(t *testing.T) {
	t.Setenv("GOOGLE_GENERAL_CLIENT_ID", "shared")
	t.Setenv("GOOGLE_WEDDING_CLIENT_ID", "shared")
	if _, err := isolatedServers(nil, "legacy", []byte("master"), nil); err == nil {
		t.Fatal("shared Google client allowed")
	}
	t.Setenv("GOOGLE_WEDDING_CLIENT_ID", "wedding-client")
	t.Setenv("MONGODB_GENERAL_DATABASE", "shared-database")
	t.Setenv("MONGODB_WEDDING_DATABASE", "shared-database")
	if _, err := isolatedServers(nil, "legacy", []byte("master"), nil); err == nil {
		t.Fatal("shared account database allowed")
	}
}

func TestIndependentProductAccountsWithSameGoogleIdentity(t *testing.T) {
	t.Setenv("GOOGLE_GENERAL_CLIENT_ID", "events-client")
	t.Setenv("GOOGLE_WEDDING_CLIENT_ID", "weddings-client")
	dbs := []*server{
		{product: "general", db: testutil.Mongo(t), secret: productSecret([]byte("master"), "general")},
		{product: "wedding", db: testutil.Mongo(t), secret: productSecret([]byte("master"), "wedding")},
	}
	var accounts []struct {
		User  core.User `json:"user"`
		Token string    `json:"token"`
	}
	for _, s := range dbs {
		if err := mongostore.EnsureIndexes(context.Background(), s.db); err != nil {
			t.Fatal(err)
		}
		s.verifyGoogle = func(_ context.Context, _ string, audience string) (*idtoken.Payload, error) {
			if audience != googleClientID(s.product) {
				t.Fatal("wrong Google audience")
			}
			return &idtoken.Payload{Subject: "same-google-person", Claims: map[string]any{"email": "same@gmail.com", "email_verified": true, "name": "Same Person"}}, nil
		}
		w := httptest.NewRecorder()
		s.googleLogin(w, httptest.NewRequest("POST", "/auth/google", strings.NewReader(`{"idToken":"verified","portal":"`+s.product+`"}`)))
		if w.Code != 200 {
			t.Fatalf("%s login: %d %s", s.product, w.Code, w.Body.String())
		}
		var account struct {
			User  core.User `json:"user"`
			Token string    `json:"token"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &account); err != nil {
			t.Fatal(err)
		}
		if account.User.Product != s.product {
			t.Fatal("account has no product boundary")
		}
		accounts = append(accounts, account)
		if err := s.ensureDemos(context.Background(), account.User); err != nil {
			t.Fatal(err)
		}
	}
	if accounts[0].User.ID == accounts[1].User.ID {
		t.Fatal("Google linked accounts across products")
	}
	for i, s := range dbs {
		r := httptest.NewRequest("GET", "/auth/me", nil)
		r.Header.Set("Authorization", "Bearer "+accounts[1-i].Token)
		if _, err := s.user(r); err == nil {
			t.Fatal("cross-product session accepted")
		}
		count, err := s.db.Collection("events").CountDocuments(context.Background(), map[string]any{"kind": dbs[1-i].product})
		if err != nil || count != 0 {
			t.Fatal("foreign product demo created")
		}
	}
}
