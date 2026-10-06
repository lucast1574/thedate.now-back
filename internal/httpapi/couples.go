package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
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
	if !validEmail(in.Email) || in.Email == adminEmail() {
		bad(w, 400, "Invalid email")
		return
	}
	if len(e.CoupleUserIDs) >= 2 {
		bad(w, 409, "This wedding already has two couple accounts")
		return
	}
	now := time.Now().UTC()
	token, err := randomToken()
	if err != nil {
		bad(w, 409, "Could not create invitation; check active couple slots")
		return
	}
	invite := core.CoupleInvite{ID: uuid.NewString(), EventID: e.ID, Email: in.Email, TokenHash: hashToken(token), ExpiresAt: now.Add(7 * 24 * time.Hour), Accepted: false}
	if err = s.couples().Invite(r.Context(), e.ID, invite); err != nil {
		bad(w, 409, "Could not create invitation; check active couple slots")
		return
	}
	reply(w, 201, map[string]any{"email": in.Email, "expiresAt": invite.ExpiresAt, "url": "https://studio.save.thedate.now/join/" + token})
}

func (s *server) coupleInviteDetails(w http.ResponseWriter, r *http.Request) {
	e, invite, err := (mongostore.Couples{DB: s.db}).Invite(r.Context(), "", hashToken(r.PathValue("token")))
	if err != nil || invite.TokenHash == "" || invite.Accepted || !invite.ExpiresAt.After(time.Now().UTC()) || e.Kind != "wedding" || e.PaymentStatus != "paid" {
		bad(w, 404, "Invitation not found")
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
	hash := hashToken(r.PathValue("token"))
	e, _, err := (mongostore.Couples{DB: s.db}).Invite(r.Context(), "", hash)
	if err != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	if err = s.couples().Accept(r.Context(), e.ID, hash, u); err != nil {
		bad(w, 403, "Sign in with the invited verified Google account; check available couple slots")
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
	if e.Kind != "wedding" || e.IsDemo || e.PaymentStatus != "paid" || !core.UserCanCreate("wedding", u) {
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
	if len(in.Name) < 2 || len(in.Name) > 120 || !validEmail(in.Email) || in.Email == adminEmail() || len(in.Password) < 10 || len(in.Password) > 72 {
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
	if err = s.couples().Attach(r.Context(), e.ID, couple.ID); err != nil {
		_, _ = s.db.Collection("users").DeleteOne(r.Context(), bson.M{"_id": couple.ID})
		bad(w, 409, "This wedding already has two couple accounts or active invitations")
		return
	}

	reply(w, 201, map[string]any{"user": couple, "eventId": e.ID})
}

func (s *server) couples() application.Couples {
	return application.Couples{Repository: mongostore.Couples{DB: s.db}, Now: func() time.Time { return time.Now().UTC() }}
}
