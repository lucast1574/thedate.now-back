package httpapi

import (
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"strings"
)

type rsvpInput struct {
	Companions *[]core.Person `json:"companions"`
	Response   string         `json:"response"`
	Reason     string         `json:"reason"`
}

func (s *server) rsvpDetails(w http.ResponseWriter, r *http.Request) {
	var g core.Guest
	if err := s.db.Collection("guests").FindOne(r.Context(), bson.M{"inviteToken": r.PathValue("token")}).Decode(&g); err != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	var e core.Event
	if err := s.db.Collection("events").FindOne(r.Context(), bson.M{"_id": g.EventID, "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}}).Decode(&e); err != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	g = core.WithResponse(e, g)
	s.completeLegacyEvent(r.Context(), &e)
	reply(w, 200, map[string]any{"event": publicEventView(e), "guestName": core.GuestFullName(g), "seats": g.Seats, "companions": g.Companions, "attendingSeats": g.AttendingSeats, "partyRegistered": g.PartyRegistered, "response": g.Response, "maybeReason": g.MaybeReason, "maybeExpiresAt": g.MaybeExpiresAt, "eventTitle": e.Title, "kind": e.Kind, "slug": e.Slug, "startAt": e.StartAt, "timeZone": e.TimeZone, "organizer": e.Organizer, "location": e.Location, "isVirtual": e.IsVirtual, "mapUrl": e.MapURL, "virtualUrl": e.VirtualURL})
}

func (s *server) rsvp(w http.ResponseWriter, r *http.Request) {
	var in rsvpInput
	if decode(r, &in) != nil || (in.Response != "going" && in.Response != "not_going" && in.Response != "maybe") || (in.Response == "maybe" && len(strings.TrimSpace(in.Reason)) < 3) || len(in.Reason) > 2000 {
		bad(w, 400, "Choose going, not going, or maybe with a reason")
		return
	}
	var g core.Guest
	if err := s.db.Collection("guests").FindOne(r.Context(), bson.M{"inviteToken": r.PathValue("token")}).Decode(&g); err != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	var e core.Event
	if err := s.db.Collection("events").FindOne(r.Context(), bson.M{"_id": g.EventID, "paymentStatus": "paid"}).Decode(&e); err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if in.Companions != nil && !core.ValidParty(*in.Companions, g.Seats) {
		bad(w, 400, "Check companion names and the allowed party size")
		return
	}
	if e.PublishedAt == nil {
		bad(w, 403, "Invitation is not published")
		return
	}
	if err := s.responses().ApplyParty(r.Context(), e.ID, g, in.Response, in.Reason, "", in.Companions); err != nil {
		if errors.Is(err, errCapacity) {
			bad(w, 409, "There are not enough seats available")
		} else {
			bad(w, 500, "Could not save response")
		}
		return
	}
	reply(w, 200, map[string]string{"status": "saved"})
}
