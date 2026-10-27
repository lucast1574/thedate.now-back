package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"net/http"
	"time"
)

// Activate the same document: media keys, canvases and unsent work keep their identity.
func (s *server) activateDemo(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	e, err := s.managedEvent(r)
	if err != nil || e.OwnerID != u.ID {
		bad(w, 404, "Demo not found")
		return
	}
	if !e.IsDemo {
		reply(w, 200, e)
		return
	}
	var in core.EventInput
	if decode(r, &in) != nil || in.Kind != e.Kind || core.ValidateEvent(in, true) != nil || in.InitialDesign != nil {
		bad(w, 400, "Invalid event details")
		return
	}
	in = core.NormalizeEvent(in)
	if in.MaybeHoldHours == 0 {
		in.MaybeHoldHours = 48
	}
	status := "unpaid"
	if u.Role == "admin" {
		status = "paid"
		if s.audit(r, u, "courtesy-activate", e.ID, "Administrator demo activation") != nil {
			bad(w, 500, "Could not audit courtesy")
			return
		}
	}
	set := bson.M{"isDemo": false, "paymentStatus": status, "slug": in.Slug, "title": in.Title, "startAt": in.StartAt, "timeZone": in.TimeZone, "organizer": in.Organizer, "location": in.Location, "isVirtual": in.IsVirtual, "mapUrl": in.MapURL, "virtualUrl": in.VirtualURL, "capacity": in.Capacity, "capacityUnlimited": in.CapacityUnlimited, "maybeHoldHours": in.MaybeHoldHours, "invitationMessage": in.InvitationMessage, "updatedAt": time.Now().UTC()}
	if in.Celebration != nil {
		set["celebration"] = in.Celebration
	}
	var activated core.Event
	err = s.db.Collection("events").FindOneAndUpdate(r.Context(), bson.M{"_id": e.ID, "ownerId": u.ID, "isDemo": true}, bson.M{"$set": set}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&activated)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			bad(w, 409, "That invitation address is already taken")
		} else {
			bad(w, 409, "Demo changed; reload before continuing")
		}
		return
	}
	reply(w, 200, activated)
}
