package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAccountDeletionErasesOwnDataAndPreservesOtherOwners(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	owner := core.User{ID: "owner", Email: "owner@example.test", Role: "user"}
	other := core.User{ID: "other", Email: "other@example.test", Role: "user"}
	for _, u := range []core.User{owner, other} {
		if _, err := db.Collection("users").InsertOne(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	for _, e := range []core.Event{
		{ID: "owned", OwnerID: owner.ID, Kind: "general", Slug: "owned", PaymentStatus: "paid", PublishedAt: &now},
		{ID: "shared", OwnerID: other.ID, CoupleUserIDs: []string{owner.ID, other.ID}, CoupleInvites: map[string]core.CoupleInvite{"mine": {Email: owner.Email}, "theirs": {Email: other.Email}}},
		{ID: "unrelated", OwnerID: other.ID, CoupleVersion: 7, CoupleInvites: map[string]core.CoupleInvite{"theirs": {Email: other.Email}}},
	} {
		if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"guests", "deletedGuestBackups", "responseNotices", "mobileDeliveries"} {
		if _, err := db.Collection(name).InsertOne(ctx, bson.M{"_id": "owned-data", "eventId": "owned"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Collection(name).InsertOne(ctx, bson.M{"_id": "other-data", "eventId": "shared"}); err != nil {
			t.Fatal(err)
		}
	}
	for name, doc := range map[string]bson.M{
		"profilePhotos":  {"_id": owner.ID, "photo": []byte("private")},
		"supportTickets": {"_id": "ticket", "requesterId": owner.ID},
		"mobileDevices":  {"_id": "device", "userId": owner.ID},
	} {
		if _, err := db.Collection(name).InsertOne(ctx, doc); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Collection("mobileDeliveries").InsertOne(ctx, bson.M{"_id": "device-delivery", "deviceId": "device"}); err != nil {
		t.Fatal(err)
	}
	token, _ := s.sign(owner, "password")
	request := func(method, path, auth string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("Authorization", "Bearer "+auth)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		return w
	}
	if w := request("DELETE", "/profile", ""); w.Code != 401 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("DELETE", "/profile", token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("GET", "/auth/me", token); w.Code != 401 {
		t.Fatal("old session still valid", w.Code)
	}
	for _, name := range []string{"users", "profilePhotos"} {
		if n, _ := db.Collection(name).CountDocuments(ctx, bson.M{"_id": owner.ID}); n != 0 {
			t.Fatal(name, "account remains")
		}
	}
	for _, name := range []string{"guests", "deletedGuestBackups", "responseNotices", "mobileDeliveries"} {
		if n, _ := db.Collection(name).CountDocuments(ctx, bson.M{"eventId": "owned"}); n != 0 {
			t.Fatal(name, "owned data remains")
		}
		if n, _ := db.Collection(name).CountDocuments(ctx, bson.M{"eventId": "shared"}); n != 1 {
			t.Fatal(name, "other owner's data deleted")
		}
	}
	for _, name := range []string{"supportTickets", "mobileDevices"} {
		if n, _ := db.Collection(name).CountDocuments(ctx, bson.M{}); n != 0 {
			t.Fatal(name, "private data remains")
		}
	}
	var shared core.Event
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": "shared"}).Decode(&shared); err != nil {
		t.Fatal(err)
	}
	if len(shared.CoupleUserIDs) != 1 || shared.CoupleUserIDs[0] != other.ID || len(shared.CoupleInvites) != 1 || shared.CoupleInvites["theirs"].Email != other.Email {
		t.Fatal("membership cleanup", shared)
	}
	var unrelated core.Event
	if err := db.Collection("events").FindOne(ctx, bson.M{"_id": "unrelated"}).Decode(&unrelated); err != nil || unrelated.CoupleVersion != 7 {
		t.Fatal("unrelated event changed", err, unrelated.CoupleVersion)
	}
}

func TestAccountDeletionResumesAfterPublicationLease(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	s := &server{db: db, secret: []byte(strings.Repeat("x", 32))}
	u := core.User{ID: "owner", Email: "owner@example.test", Role: "user"}
	_, _ = db.Collection("users").InsertOne(ctx, u)
	now := time.Now().UTC()
	_, _ = db.Collection("events").InsertOne(ctx, core.Event{ID: "event", OwnerID: u.ID, Kind: "general", Slug: "event", PaymentStatus: "paid", PublishedAt: &now})
	_, _ = db.Collection("deployments").InsertOne(ctx, bson.M{"_id": "event", "phase": "deploying", "leaseUntil": now.Add(time.Minute)})
	token, _ := s.sign(u, "password")
	r := httptest.NewRequest("DELETE", "/profile", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "/auth/me", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("pending account still has access")
	}
	w = httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/public/events/general/event", nil))
	if w.Code != 404 {
		t.Fatal("pending invitation still public", w.Code)
	}
	_, _ = db.Collection("deployments").UpdateOne(ctx, bson.M{"_id": "event"}, bson.M{"$unset": bson.M{"leaseUntil": ""}})
	_, _ = db.Collection("users").UpdateOne(ctx, bson.M{"_id": u.ID}, bson.M{"$unset": bson.M{"deletionNextAttempt": ""}})
	if err := s.processAccountDeletion(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := db.Collection("users").CountDocuments(ctx, bson.M{"_id": u.ID}); n != 0 {
		t.Fatal("retry did not finish")
	}
	if n, _ := db.Collection("events").CountDocuments(ctx, bson.M{}); n != 0 {
		t.Fatal("event remains")
	}
	if err := (mongostore.AccountDeletion{DB: db}).Request(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
}
