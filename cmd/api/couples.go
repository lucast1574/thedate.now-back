package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type coupleEmail struct {
	Email string `json:"email"`
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *server) inviteCouple(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.Kind != "wedding" || e.PaymentStatus != "paid" || e.OwnerID != u.ID {
		bad(w, 403, "Only the planner can invite couples after payment")
		return
	}
	var in coupleEmail
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid email")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !strings.Contains(in.Email, "@") {
		bad(w, 400, "Invalid email")
		return
	}
	if len(e.CoupleUserIDs) >= 2 {
		bad(w, 409, "This wedding already has two couple accounts")
		return
	}
	now := time.Now().UTC()
	count, err := s.db.Collection("coupleInvites").CountDocuments(r.Context(), bson.M{"eventId": e.ID, "$or": bson.A{bson.M{"accepted": true}, bson.M{"expiresAt": bson.M{"$gt": now}}}})
	if err != nil {
		bad(w, 500, "Could not check invitations")
		return
	}
	if count >= 2 {
		bad(w, 409, "Two active couple invitations already exist")
		return
	}
	token, err := randomToken()
	if err != nil {
		bad(w, 500, "Could not create invitation")
		return
	}
	invite := core.CoupleInvite{ID: uuid.NewString(), EventID: e.ID, Email: in.Email, TokenHash: hashToken(token), ExpiresAt: now.Add(7 * 24 * time.Hour), Accepted: false}
	if _, err = s.db.Collection("coupleInvites").InsertOne(r.Context(), invite); err != nil {
		bad(w, 500, "Could not create invitation")
		return
	}
	reply(w, 201, map[string]any{"email": in.Email, "expiresAt": invite.ExpiresAt, "url": "https://studio.save.thedate.now/join/" + token})
}

func (s *server) coupleInviteDetails(w http.ResponseWriter, r *http.Request) {
	var invite core.CoupleInvite
	if err := s.db.Collection("coupleInvites").FindOne(r.Context(), bson.M{"tokenHash": hashToken(r.PathValue("token")), "accepted": false, "expiresAt": bson.M{"$gt": time.Now().UTC()}}).Decode(&invite); err != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	var e core.Event
	if err := s.db.Collection("events").FindOne(r.Context(), bson.M{"_id": invite.EventID, "kind": "wedding", "paymentStatus": "paid"}).Decode(&e); err != nil {
		bad(w, 404, "Wedding not found")
		return
	}
	reply(w, 200, map[string]string{"email": invite.Email, "eventTitle": e.Title})
}

func (s *server) acceptCouple(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	var invite core.CoupleInvite
	if err := s.db.Collection("coupleInvites").FindOne(r.Context(), bson.M{"tokenHash": hashToken(r.PathValue("token")), "accepted": false, "expiresAt": bson.M{"$gt": time.Now().UTC()}}).Decode(&invite); err != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	if invite.Email != u.Email {
		bad(w, 403, "Sign in with the invited email address")
		return
	}
	var e core.Event
	if err := s.db.Collection("events").FindOne(r.Context(), bson.M{"_id": invite.EventID, "kind": "wedding", "paymentStatus": "paid"}).Decode(&e); err != nil {
		bad(w, 404, "Wedding not found")
		return
	}
	if len(e.CoupleUserIDs) >= 2 {
		bad(w, 409, "This wedding already has two couple accounts")
		return
	}
	if _, err = s.db.Collection("events").UpdateByID(r.Context(), e.ID, bson.M{"$addToSet": bson.M{"coupleUserIds": u.ID}}); err != nil {
		bad(w, 500, "Could not grant access")
		return
	}
	if _, err = s.db.Collection("coupleInvites").UpdateByID(r.Context(), invite.ID, bson.M{"$set": bson.M{"accepted": true}}); err != nil {
		bad(w, 500, "Could not complete invitation")
		return
	}
	reply(w, 200, map[string]string{"status": "accepted", "eventId": e.ID})
}
