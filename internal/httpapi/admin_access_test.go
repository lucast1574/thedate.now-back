package httpapi

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminCourtesyRolesAndScopedCollaborator(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	t.Setenv("ADMIN_EMAIL", "owner@gmail.com")
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	root := core.User{ID: uuid.NewString(), Email: "owner@gmail.com", Role: "admin", GoogleSub: "root", GoogleAuthoritative: true, IdentityPolicy: 1}
	local := core.User{ID: uuid.NewString(), Email: "local@example.test", Role: "organizer"}
	guest := core.User{ID: uuid.NewString(), Email: "guest@example.test", Role: "organizer"}
	verified := core.User{ID: uuid.NewString(), Email: "verified@gmail.com", Role: "planner", GoogleSub: "verified", GoogleAuthoritative: true, IdentityPolicy: 1}
	for _, u := range []core.User{root, local, guest, verified} {
		if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	request := func(u core.User, method, path, body string) *httptest.ResponseRecorder {
		token, err := s.sign(u, "password")
		if u.GoogleSub != "" {
			token, err = s.sign(u, "google")
		}
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/admin/overview", "/admin/users", "/admin/withdrawals"} {
		if w := request(local, "GET", path, ""); w.Code != 403 {
			t.Fatalf("nonadmin access %s: %d", path, w.Code)
		}
	}
	event := core.Event{ID: uuid.NewString(), Kind: "general", OwnerID: local.ID, PaymentStatus: "unpaid"}
	_, _ = db.Collection("events").InsertOne(ctx, event)
	if w := request(local, "POST", "/admin/events/"+event.ID+"/courtesy", `{"reason":"family"}`); w.Code != 403 {
		t.Fatal("nonadmin courtesy")
	}
	if w := request(root, "POST", "/admin/events/"+event.ID+"/courtesy", `{"reason":"family"}`); w.Code != 200 {
		t.Fatalf("courtesy: %d %s", w.Code, w.Body.String())
	}
	_ = db.Collection("events").FindOne(ctx, bson.M{"_id": event.ID}).Decode(&event)
	if event.PaymentSource != "courtesy" || event.Payment != nil || event.PaymentStatus != "paid" {
		t.Fatal("fake income recorded")
	}
	if w := request(root, "PATCH", "/admin/users/"+root.ID+"/role", `{"role":"organizer"}`); w.Code != 403 {
		t.Fatal("root demoted")
	}
	if w := request(root, "PATCH", "/admin/users/"+local.ID+"/role", `{"role":"admin"}`); w.Code != 403 {
		t.Fatal("password admin granted")
	}
	if w := request(root, "PATCH", "/admin/users/"+verified.ID+"/role", `{"role":"admin"}`); w.Code != 200 {
		t.Fatalf("verified promotion: %d %s", w.Code, w.Body.String())
	}
	if w := request(verified, "GET", "/auth/me", ""); w.Code != 401 {
		t.Fatal("role change did not revoke old token")
	}
	_ = db.Collection("users").FindOne(ctx, bson.M{"_id": verified.ID}).Decode(&verified)
	if w := request(verified, "GET", "/admin/overview", ""); w.Code != 200 {
		t.Fatal("delegated verified admin unusable")
	}
	token := strings.Repeat("a", 48)
	inv := core.CoupleInvite{ID: uuid.NewString(), Email: guest.Email, EventID: event.ID, TokenHash: hashToken(token), ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.couples().Invite(ctx, event.ID, inv); err != nil {
		t.Fatal(err)
	}
	if w := request(local, "POST", "/access-invites/"+token+"/accept", ""); w.Code != 403 {
		t.Fatal("wrong recipient accepted")
	}
	if w := request(guest, "POST", "/access-invites/"+token+"/accept", ""); w.Code != 200 {
		t.Fatalf("recipient denied: %d %s", w.Code, w.Body.String())
	}
	if w := request(guest, "GET", "/events/"+event.ID+"/collaborators", ""); w.Code != 404 {
		t.Fatal("collaborator manages access")
	}
	if w := request(guest, "GET", "/events/"+event.ID+"/guests", ""); w.Code != 200 {
		t.Fatalf("member cannot plan: %d", w.Code)
	}
	if w := request(local, "DELETE", "/events/"+event.ID+"/collaborators/"+guest.ID, ""); w.Code != 200 {
		t.Fatal("revoke failed")
	}
	if w := request(guest, "GET", "/events/"+event.ID+"/guests", ""); w.Code != 404 {
		t.Fatal("revoked membership survives")
	}
	snapshot, err := (mongostore.Couples{DB: db}).Snapshot(ctx, event.ID)
	if err != nil || len(snapshot.CoupleUserIDs) != 0 {
		t.Fatal("membership retained")
	}
	w := request(root, "GET", "/admin/overview", "")
	var overview struct {
		Products map[string]productRevenue `json:"products"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &overview)
	if overview.Products["general"].Gross != 0 || overview.Products["general"].Courtesies != 1 {
		t.Fatal("courtesy income counted")
	}
}
