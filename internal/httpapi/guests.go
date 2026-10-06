package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"regexp"
	"time"
)

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

func (s *server) addGuest(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 403, "Pay before adding guests")
		return
	}
	var in guestInput
	if decode(r, &in) != nil || !validGuest(in) {
		bad(w, 400, "A name, international phone number and 1-20 seats are required")
		return
	}
	token, err := randomToken()
	if err != nil {
		bad(w, 500, "Could not create invitation")
		return
	}
	g := guestFromInput(in, uuid.NewString(), e.ID, token)
	g.CreatedAt = time.Now().UTC()
	if _, err = s.db.Collection("guests").InsertOne(r.Context(), g); err != nil {
		bad(w, 500, "Could not add guest")
		return
	}
	reply(w, 201, g)
}

func (s *server) listGuests(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo {
		reply(w, 200, []core.Guest{})
		return
	}
	cur, err := s.db.Collection("guests").Find(r.Context(), bson.M{"eventId": e.ID})
	if err != nil {
		bad(w, 500, "Could not list guests")
		return
	}
	defer cur.Close(r.Context())
	guests := []core.Guest{}
	if err = cur.All(r.Context(), &guests); err != nil {
		bad(w, 500, "Could not list guests")
		return
	}
	e, err = (mongostore.Responses{DB: s.db}).Snapshot(r.Context(), e.ID)
	if err != nil {
		bad(w, 500, "Could not load responses")
		return
	}
	views := make([]map[string]any, 0, len(guests))
	for _, guest := range guests {
		views = append(views, guestView(e, core.WithResponse(e, guest)))
	}
	reply(w, 200, views)
}

func guestView(e core.Event, g core.Guest) map[string]any {
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	return map[string]any{"id": g.ID, "name": g.Name, "phone": g.Phone, "seats": g.Seats, "lastName": g.LastName, "family": g.Family, "gender": g.Gender, "companions": g.Companions, "attendingSeats": g.AttendingSeats, "partyRegistered": g.PartyRegistered, "response": g.Response, "maybeReason": g.MaybeReason, "maybeExpiresAt": g.MaybeExpiresAt, "sentAt": g.SentAt, "invitationUrl": "https://" + host + "/rsvp/" + g.InviteToken}
}
