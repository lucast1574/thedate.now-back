package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"testing"
)

func TestRoleChangeLegacyVersionAndStaleWrites(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	for _, legacy := range []bool{false, true} {
		id := "current"
		if legacy {
			id = "legacy"
		}
		doc := bson.M{"_id": id, "role": "organizer"}
		if !legacy {
			doc["tokenVersion"] = 0
		}
		if _, err := db.Collection("users").InsertOne(ctx, doc); err != nil {
			t.Fatal(err)
		}
		original := core.User{ID: id, Role: "organizer"}
		changed, err := ChangeUserRole(ctx, db, original, "planner")
		if err != nil || !changed {
			t.Fatalf("legacy=%v changed=%v error=%v", legacy, changed, err)
		}
		var current core.User
		if err := db.Collection("users").FindOne(ctx, bson.M{"_id": id}).Decode(&current); err != nil {
			t.Fatal(err)
		}
		if current.Role != "planner" || current.TokenVersion != 1 {
			t.Fatal("role or revocation version was not saved")
		}
		changed, err = ChangeUserRole(ctx, db, original, "admin")
		if err != nil || changed {
			t.Fatal("stale role/version update accepted")
		}
		current.TokenVersion = 0
		changed, err = ChangeUserRole(ctx, db, current, "admin")
		if err != nil || changed {
			t.Fatal("stale zero version accepted")
		}
	}
}
