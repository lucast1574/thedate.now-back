package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/api/idtoken"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultAffiliateLinksRegistrationAttributionAndCredit(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	t.Setenv("ADMIN_EMAIL", "owner@gmail.com")
	if err := mongostore.EnsureIndexes(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	referrer := decodeLogin(t, routeRequest(t, s, "", "POST", "/auth/register", `{"email":"affiliate@example.test","password":"test-password-123","name":"Affiliate","role":"planner"}`, 201))
	type overview struct {
		Enabled   bool   `json:"enabled"`
		Code      string `json:"code"`
		Available int64  `json:"availableCents"`
	}
	readWallet := func() overview {
		t.Helper()
		var result overview
		if err := json.Unmarshal(routeRequest(t, s, referrer.Token, "GET", "/affiliates", "", 200).Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := readWallet()
	if !first.Enabled || !affiliatePattern.MatchString(first.Code) || first.Available != 0 {
		t.Fatal("new account requires affiliate activation")
	}
	if second := readWallet(); second.Code != first.Code {
		t.Fatal("link changed on reload")
	}
	buyer := decodeLogin(t, routeRequest(t, s, "", "POST", "/auth/register", `{"email":"buyer@example.test","password":"test-password-123","name":"Buyer","role":"organizer","referralCode":"`+first.Code+`"}`, 201))
	var stored core.User
	if err := db.Collection("users").FindOne(ctx, bson.M{"_id": buyer.User.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.ReferredBy != referrer.User.ID {
		t.Fatal("default link did not attribute registration")
	}
	repo := mongostore.Affiliates{DB: db}
	event := core.Event{ID: "first", OwnerID: stored.ID, Kind: "general", PaymentSource: "stripe", Payment: &core.Payment{SessionID: "first-payment", AmountCents: 100000, Currency: "USD", Live: true}}
	for range 2 {
		if err := repo.Convert(ctx, stored.ID, event); err != nil {
			t.Fatal(err)
		}
	}
	if got := readWallet(); got.Available != 10000 || got.Code != first.Code {
		t.Fatal("commission missing, duplicated or link changed")
	}
	routeRequest(t, s, referrer.Token, "POST", "/affiliates/withdrawals", `{"amountCents":4999,"method":"bank","details":"test account"}`, 400)
	routeRequest(t, s, referrer.Token, "POST", "/affiliates/withdrawals", `{"amountCents":20000,"method":"bank","details":"test account"}`, 409)
	routeRequest(t, s, referrer.Token, "POST", "/affiliates/withdrawals", `{"amountCents":5000,"method":"bank","details":"test account"}`, 201)
	if readWallet().Available != 5000 {
		t.Fatal("withdrawal reservation not deducted")
	}
	if got := s.referredBy(httptest.NewRequest("GET", "/", nil), "invalid"); got != "" {
		t.Fatal("invalid link attributed")
	}
}

func TestGoogleAndLegacyAccountsGetDefaultLinks(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	t.Setenv("ADMIN_EMAIL", "owner@gmail.com")
	t.Setenv("GOOGLE_CLIENT_ID", "test-client")
	if err := mongostore.EnsureIndexes(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32)), verifyGoogle: func(context.Context, string, string) (*idtoken.Payload, error) {
		return &idtoken.Payload{Subject: "google-user", Claims: map[string]any{"email": "new@gmail.com", "email_verified": true, "name": "New"}}, nil
	}}
	google := decodeLogin(t, routeRequest(t, s, "", "POST", "/auth/google", `{"idToken":"test","portal":"wedding"}`, 200))
	wallet, err := (mongostore.Affiliates{DB: db}).Wallet(ctx, google.User.ID)
	if err != nil || !wallet.Enabled || !affiliatePattern.MatchString(wallet.Code) {
		t.Fatal("Google links not enabled")
	}
	code := "0123456789abcdef01234567"
	for _, u := range []bson.M{{"_id": "legacy", "email": "legacy@example.test", "role": "planner", "affiliateCode": code, "affiliateEnabled": false, "affiliateBalanceCents": int64(1234)}, {"_id": "no-code", "email": "no-code@example.test", "role": "organizer"}, {"_id": "admin", "email": "owner@gmail.com", "role": "admin"}} {
		if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	if err := mongostore.EnsureAffiliateDefaults(ctx, db); err != nil {
		t.Fatal(err)
	}
	wallet, err = (mongostore.Affiliates{DB: db}).Wallet(ctx, "legacy")
	if err != nil || !wallet.Enabled || wallet.Code != code || wallet.Balance != 1234 {
		t.Fatal("legacy migration changed existing link/balance")
	}
	wallet, err = (mongostore.Affiliates{DB: db}).Wallet(ctx, "no-code")
	if err != nil || !wallet.Enabled || !affiliatePattern.MatchString(wallet.Code) {
		t.Fatal("legacy account did not receive link")
	}
	wallet, err = (mongostore.Affiliates{DB: db}).Wallet(ctx, "admin")
	if err != nil || wallet.Enabled || wallet.Code != "" {
		t.Fatal("courtesy administrator entered commission program")
	}
}
