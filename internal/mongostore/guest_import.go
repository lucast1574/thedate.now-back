package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"strings"
)

type GuestImports struct{ DB *mongo.Database }
type GuestImportResult struct{ Added, Updated, Review bool }

// All new creators use the same deterministic ID, including concurrent manual adds.
// The event/phone lookup also finds legacy guests created with random IDs.
func (s GuestImports) Add(ctx context.Context, guest core.Guest) (bool, error) {
	result, err := s.DB.Collection("guests").UpdateOne(ctx,
		bson.M{"eventId": guest.EventID, "phone": guest.Phone},
		bson.M{"$setOnInsert": guest}, options.UpdateOne().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		var existing core.Guest
		err = s.DB.Collection("guests").FindOne(ctx, bson.M{"eventId": guest.EventID, "phone": guest.Phone}).Decode(&existing)
		return false, err
	}
	if err != nil {
		return false, err
	}
	return result.UpsertedCount == 1, nil
}

func (s GuestImports) Apply(ctx context.Context, guest core.Guest) (GuestImportResult, error) {
	added, err := s.Add(ctx, guest)
	if err != nil || added {
		return GuestImportResult{Added: added}, err
	}
	var existing core.Guest
	if err = s.DB.Collection("guests").FindOne(ctx, bson.M{"eventId": guest.EventID, "phone": guest.Phone}).Decode(&existing); err != nil {
		return GuestImportResult{}, err
	}
	merged, review := core.MergeGuestImport(existing, guest)
	set := bson.M{"name": merged.Name}
	if strings.TrimSpace(guest.LastName) != "" {
		set["lastName"] = merged.LastName
	}
	if strings.TrimSpace(guest.Family) != "" {
		set["family"] = merged.Family
	}
	if guest.Gender != "" && guest.Gender != "unspecified" {
		set["gender"] = merged.Gender
	}
	result, err := s.DB.Collection("guests").UpdateOne(ctx, bson.M{"_id": existing.ID, "eventId": guest.EventID},
		bson.M{"$set": set})
	if err != nil {
		return GuestImportResult{}, err
	}
	if result.MatchedCount != 1 {
		return GuestImportResult{}, mongo.ErrNoDocuments
	}
	return GuestImportResult{Updated: result.ModifiedCount > 0, Review: review}, nil
}
