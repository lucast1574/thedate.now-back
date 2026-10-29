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
)

func TestReimportReportsChangesPreservesOrderAndRejectsManualDuplicates(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	user := core.User{ID: "owner", Email: "owner@example.test", Role: "organizer"}
	_, _ = db.Collection("users").InsertOne(ctx, user)
	_, _ = db.Collection("events").InsertOne(ctx, core.Event{ID: "event", Kind: "general", OwnerID: user.ID, PaymentStatus: "paid"})
	s := &server{db: db, secret: []byte(strings.Repeat("s", 32))}
	token, _ := s.sign(user, "password")
	call := func(path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		s.routes().ServeHTTP(response, request)
		return response
	}
	first := `{"guests":[{"name":"Ana","phone":"+51925431535","seats":2},{"name":"Ignored duplicate","phone":"+51925431535","seats":1}]}`
	response := call("/events/event/guests/import", first)
	var result struct {
		Added, Updated, Skipped, Duplicates int
		Review                              []map[string]string
	}
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	if response.Code != 200 || result.Added != 1 || result.Updated != 0 || result.Duplicates != 1 {
		t.Fatalf("first import: %d %s", response.Code, response.Body.String())
	}
	var original core.Guest
	_ = db.Collection("guests").FindOne(ctx, bson.M{"eventId": "event"}).Decode(&original)
	second := `{"guests":[{"name":"Ana updated","phone":"+51925431535","seats":1,"family":"Amigos"},{"name":"Andre","phone":"+51908837124","seats":1}]}`
	response = call("/events/event/guests/import", second)
	result.Review = nil
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	if response.Code != 200 || result.Added != 1 || result.Updated != 1 || len(result.Review) != 1 {
		t.Fatalf("updates: %d %s", response.Code, response.Body.String())
	}
	var saved core.Guest
	_ = db.Collection("guests").FindOne(ctx, bson.M{"_id": original.ID}).Decode(&saved)
	if saved.Name != "Ana updated" || saved.Seats != 2 || saved.InviteToken != original.InviteToken || saved.QueuePosition != original.QueuePosition {
		t.Fatal("reimport changed invitation identity or seats")
	}
	response = call("/events/event/guests/import", second)
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	if response.Code != 200 || result.Added != 0 || result.Updated != 0 || result.Skipped != 2 {
		t.Fatalf("retry: %d %s", response.Code, response.Body.String())
	}
	response = call("/events/event/guests", `{"name":"Again","phone":"+51925431535","seats":1}`)
	if response.Code != 409 {
		t.Fatalf("manual duplicate allowed: %d %s", response.Code, response.Body.String())
	}
	count, _ := db.Collection("guests").CountDocuments(ctx, bson.M{"eventId": "event"})
	if count != 2 {
		t.Fatalf("duplicates: %d", count)
	}
}
