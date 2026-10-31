package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCollaboratorCanCheckDeploymentButCannotPublish(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	t.Setenv("DOKPLOY_URL", "")
	e := core.Event{ID: "shared", Kind: "wedding", Slug: "our-day", OwnerID: "owner", CoupleUserIDs: []string{"couple"}, PaymentStatus: "paid"}
	if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("deployments").InsertOne(ctx, core.Deployment{EventID: e.ID, Phase: "ready", Host: "our-day.save.thedate.now"}); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	for _, id := range []string{"couple", "stranger"} {
		u := core.User{ID: id, Email: id + "@example.com", Role: "user"}
		if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
			t.Fatal(err)
		}
		token, err := s.sign(u, "password")
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []string{"GET", "POST"} {
			path := "/events/shared/deployment"
			if method == "POST" {
				path = "/events/shared/publish"
			}
			r := httptest.NewRequest(method, path, strings.NewReader("{}"))
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.routes().ServeHTTP(w, r)
			want := 404
			if id == "couple" && method == "GET" {
				want = 200
			}
			if w.Code != want {
				t.Fatalf("%s %s: %d %s", id, method, w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "our-day.save") {
				t.Fatal("unverified public address disclosed")
			}
		}
	}
}
