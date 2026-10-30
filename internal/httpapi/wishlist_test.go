package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWishlistEndpointsScopeTokensProtectNamesAndPreserveClaims(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	owner := core.User{ID: "owner", Role: "organizer", Email: "owner@example.test"}
	db.Collection("users").InsertOne(ctx, owner)
	e := core.Event{ID: "e", Kind: "general", Slug: "celebracion", OwnerID: owner.ID, PaymentStatus: "paid", PublishedAt: &now, Wishlist: core.Wishlist{Enabled: true, Items: []core.Gift{{ID: "gift", Title: "Cafetera", URL: "https://example.com/item"}, {ID: "second", Title: "Unas copas", URL: "https://example.com/two"}}}}
	db.Collection("events").InsertOne(ctx, e)
	alice := core.Guest{ID: "private-alice-id", Name: "Alicia Privada", EventID: e.ID, InviteToken: strings.Repeat("a", 48)}
	bob := core.Guest{ID: "private-bob-id", Name: "Bruno", EventID: e.ID, InviteToken: strings.Repeat("b", 48)}
	db.Collection("guests").InsertMany(ctx, []any{alice, bob})
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	auth, err := s.sign(owner, "password")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/events/e/wishlist", "", ""); w.Code != 404 {
		t.Fatal("anonymous management", w.Code)
	}
	w := call("POST", "/public/wishlist/"+alice.InviteToken, `{"giftId":"gift"}`, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("GET", "/public/wishlist/"+bob.InviteToken, "", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), alice.Name) || strings.Contains(w.Body.String(), alice.ID) || strings.Contains(w.Body.String(), "guestId") || !strings.Contains(w.Body.String(), `"reserved":true`) {
		t.Fatal("public view leaks assignment or status", w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("guest details cached")
	}
	if w = call("POST", "/public/wishlist/"+bob.InviteToken, `{"giftId":"gift"}`, ""); w.Code != 409 {
		t.Fatal("duplicate reservation", w.Code)
	}
	if w = call("POST", "/public/wishlist/"+alice.InviteToken, `{"giftId":"second"}`, ""); w.Code != 409 {
		t.Fatal("multiple gifts per guest", w.Code)
	}
	w = call("GET", "/events/e/wishlist", "", auth)
	if w.Code != 200 || !strings.Contains(w.Body.String(), alice.Name) {
		t.Fatal("organizer cannot track gift", w.Body.String())
	}
	var data struct {
		List core.Wishlist `json:"list"`
	}
	if json.Unmarshal(w.Body.Bytes(), &data) != nil {
		t.Fatal("invalid JSON")
	}
	stale := e.Wishlist
	stale.Items = []core.Gift{}
	body, _ := json.Marshal(stale)
	if w = call("PATCH", "/events/e/wishlist", string(body), auth); w.Code != 409 {
		t.Fatal("stale write accepted", w.Code)
	}
	in := data.List
	in.Items[0].GuestID = bob.ID
	in.Items[0].Delivered = true
	body, _ = json.Marshal(in)
	if w = call("PATCH", "/events/e/wishlist", string(body), auth); w.Code != 200 {
		t.Fatal("save", w.Code, w.Body.String())
	}
	var saved core.Event
	db.Collection("events").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&saved)
	if saved.Wishlist.Items[0].GuestID != alice.ID || saved.Wishlist.Items[0].Delivered {
		t.Fatal("client forged assignment")
	}
	if w = call("POST", "/events/e/wishlist/metadata", `{"url":"https://127.0.0.1/private"}`, auth); w.Code != 422 {
		t.Fatal("private metadata destination", w.Code)
	}
	for key, value := range map[string]any{"paymentStatus": "unpaid", "publishedAt": nil, "isDemo": true} {
		db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{key: value}})
		if w = call("GET", "/public/wishlist/"+bob.InviteToken, "", ""); w.Code != 404 {
			t.Fatal("eligibility bypass", key, w.Code)
		}
		original := map[string]any{"paymentStatus": "paid", "publishedAt": now, "wishlist.enabled": true, "isDemo": false}
		db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{key: original[key]}})
	}
}
