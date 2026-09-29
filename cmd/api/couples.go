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
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
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
	if e.Kind != "wedding" || e.PaymentStatus != "paid" || (e.OwnerID != u.ID && u.Role != "admin") {
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
	if u.Role != "couple" {
		bad(w, 403, "Couple account required")
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
	result, err := s.db.Collection("events").UpdateOne(r.Context(), bson.M{"_id": e.ID, "coupleUserIds.1": bson.M{"$exists": false}}, bson.M{"$addToSet": bson.M{"coupleUserIds": u.ID}})
	if err != nil || result.MatchedCount == 0 {
		bad(w, 500, "Could not grant access")
		return
	}
	if _, err = s.db.Collection("coupleInvites").UpdateByID(r.Context(), invite.ID, bson.M{"$set": bson.M{"accepted": true}}); err != nil {
		bad(w, 500, "Could not complete invitation")
		return
	}
	reply(w, 200, map[string]string{"status": "accepted", "eventId": e.ID})
}

type coupleAccountInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *server) createCoupleAccount(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Wedding not found")
		return
	}
	if e.Kind != "wedding" || e.IsDemo || e.PaymentStatus != "paid" || (u.Role != "planner" && u.Role != "admin") {
		bad(w, 403, "Paid wedding and planner access required")
		return
	}
	var in coupleAccountInput
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid account")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if len(in.Name) < 2 || len(in.Name) > 120 || !strings.Contains(in.Email, "@") || len(in.Password) < 10 || len(in.Password) > 72 {
		bad(w, 400, "Name, email and password of 10-72 characters required")
		return
	}
	if len(e.CoupleUserIDs) >= 2 {
		bad(w, 409, "This wedding already has two couple accounts")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		bad(w, 500, "Could not create account")
		return
	}
	couple := core.User{ID: uuid.NewString(), Email: in.Email, Name: in.Name, PasswordHash: string(hash), Role: "couple", CreatedAt: time.Now().UTC()}
	if _, err = s.db.Collection("users").InsertOne(r.Context(), couple); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			bad(w, 409, "This email already has an account")
			return
		}
		bad(w, 500, "Could not create account")
		return
	}
	result, err := s.db.Collection("events").UpdateOne(r.Context(), bson.M{"_id": e.ID, "coupleUserIds.1": bson.M{"$exists": false}}, bson.M{"$addToSet": bson.M{"coupleUserIds": couple.ID}, "$set": bson.M{"updatedAt": time.Now().UTC()}})
	if err != nil || result.MatchedCount == 0 {
		_, _ = s.db.Collection("users").DeleteOne(r.Context(), bson.M{"_id": couple.ID})
		bad(w, 409, "This wedding already has two couple accounts")
		return
	}
	reply(w, 201, map[string]any{"user": couple, "eventId": e.ID})
}
