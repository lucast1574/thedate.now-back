package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *server) importGuests(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 403, "Pay before importing guests")
		return
	}
	var in struct {
		Guests []guestInput `json:"guests"`
	}
	if decode(r, &in) != nil || len(in.Guests) < 1 || len(in.Guests) > 500 {
		bad(w, 400, "Import 1-500 guests at a time")
		return
	}
	for _, row := range in.Guests {
		if !validGuest(row) {
			bad(w, 400, "Check every name, international phone and seat count before importing")
			return
		}
	}
	added, skipped := 0, 0
	for _, row := range in.Guests {
		token, err := randomToken()
		if err != nil {
			bad(w, 500, "Import interrupted; retry the file safely")
			return
		}
		key := sha256.Sum256([]byte(e.ID + ":" + row.Phone))
		g := guestFromInput(row, "import-"+hex.EncodeToString(key[:]), e.ID, token)
		g.CreatedAt = time.Now().UTC()
		result, err := s.db.Collection("guests").UpdateOne(r.Context(), bson.M{"eventId": e.ID, "phone": row.Phone}, bson.M{"$setOnInsert": g}, options.UpdateOne().SetUpsert(true))
		if mongo.IsDuplicateKeyError(err) {
			skipped++
			continue
		}
		if err != nil {
			bad(w, 500, "Import interrupted; retry the file safely")
			return
		}
		if result.UpsertedCount == 1 {
			added++
		} else {
			skipped++
		}
	}
	reply(w, 200, map[string]int{"added": added, "skipped": skipped})
}
