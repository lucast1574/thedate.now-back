package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"strings"
	"testing"
)

func TestAffiliateAPIRequiresProfessionalRoleForItsProduct(t *testing.T) {
	db := testutil.Mongo(t)
	for _, product := range []string{"wedding", "general"} {
		s := &server{db: db, product: product, secret: []byte(strings.Repeat("x", 32))}
		for _, role := range []string{"user", "planner", "organizer"} {
			u := core.User{ID: product + role, Product: product, Role: role, Email: product + role + "@example.test"}
			if _, err := db.Collection("users").InsertOne(context.Background(), u); err != nil {
				t.Fatal(err)
			}
			token, err := s.sign(u, "password")
			if err != nil {
				t.Fatal(err)
			}
			overview, withdrawal := 403, 403
			if core.ProfessionalFor(product, u) {
				overview, withdrawal = 200, 400
			}
			routeRequest(t, s, token, "GET", "/affiliates", "", overview)
			routeRequest(t, s, token, "POST", "/affiliates/withdrawals", `{}`, withdrawal)
		}
		if s.professionalAffiliate(core.User{Role: "admin"}) {
			t.Fatal("admin entered professional program")
		}
	}
}
