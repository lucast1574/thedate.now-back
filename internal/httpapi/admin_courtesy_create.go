package httpapi

import (
	"net/http"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// A courtesy on the account's demo funds exactly one real invitation. Convert
// that same document atomically so parallel creates cannot spend it twice.
func (s *server) createCourtesyEvent(w http.ResponseWriter, r *http.Request, next core.Event) bool {
	filter := bson.M{"ownerId": next.OwnerID, "kind": next.Kind, "isDemo": true, "paymentStatus": "paid", "paymentSource": "courtesy", "deletionRequested": bson.M{"$ne": true}}
	var demo core.Event
	err := s.db.Collection("events").FindOne(r.Context(), filter).Decode(&demo)
	if err == mongo.ErrNoDocuments {
		return false
	}
	if err != nil {
		bad(w, 500, "Could not verify courtesy")
		return true
	}
	next.ID, next.PaymentStatus, next.PaymentSource = demo.ID, "paid", "courtesy"
	data, err := bson.Marshal(next)
	if err != nil {
		bad(w, 500, "Could not prepare courtesy invitation")
		return true
	}
	var set bson.M
	if bson.Unmarshal(data, &set) != nil {
		bad(w, 500, "Could not prepare courtesy invitation")
		return true
	}
	delete(set, "_id")
	set["isDemo"] = false
	filter["_id"] = demo.ID
	filter["designVersion"] = demo.DesignVersion
	err = s.db.Collection("events").FindOneAndUpdate(r.Context(), filter, bson.M{"$set": set}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&next)
	if err != nil {
		bad(w, 409, "La cortesía o el diseño cambió. Recarga tus invitaciones antes de continuar.")
		return true
	}
	reply(w, 201, next)
	return true
}
