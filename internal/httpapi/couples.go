package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
)

var accessTokenPattern = regexp.MustCompile(`^[a-f0-9]{48}$`)

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
	if e.PaymentStatus != "paid" || e.IsDemo || (e.OwnerID != u.ID && u.Role != "admin") {
		bad(w, 403, "Only the owner or administrator can invite collaborators after payment")
		return
	}
	var in coupleEmail
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid email")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !validEmail(in.Email) || in.Email == u.Email {
		bad(w, 400, "Invalid email")
		return
	}
	if len(e.CoupleUserIDs) >= 2 {
		bad(w, 409, "This event already has two collaborators")
		return
	}
	if !mailConfigured() {
		bad(w, 503, "Email invitations are not configured")
		return
	}
	if s.isEventMember(r.Context(), e, in.Email) {
		bad(w, 409, "This account already has access")
		return
	}
	now := time.Now().UTC()
	token, err := randomToken()
	if err != nil {
		bad(w, 409, "Could not create invitation; check active collaborator slots")
		return
	}
	invite := core.CoupleInvite{ID: uuid.NewString(), EventID: e.ID, Email: in.Email, TokenHash: hashToken(token), ExpiresAt: now.Add(7 * 24 * time.Hour), Accepted: false, Delivery: "sending"}
	if err = s.couples().Invite(r.Context(), e.ID, invite); err != nil {
		bad(w, 409, "Could not create invitation; check active collaborator slots")
		return
	}
	delivery := s.deliverAccess(r.Context(), e, invite, token, u.Name)
	reply(w, 201, map[string]any{"delivery": delivery, "email": in.Email, "expiresAt": invite.ExpiresAt, "url": studioFor(e.Kind) + "/join/" + token})
}

func (s *server) coupleInviteDetails(w http.ResponseWriter, r *http.Request) {
	if !accessTokenPattern.MatchString(r.PathValue("token")) {
		bad(w, 404, "Invitation not found")
		return
	}
	e, invite, err := (mongostore.Couples{DB: s.db}).Invite(r.Context(), "", hashToken(r.PathValue("token")))
	if err != nil || invite.TokenHash == "" || invite.Accepted || !invite.ExpiresAt.After(time.Now().UTC()) || e.PaymentStatus != "paid" {
		bad(w, 404, "Invitation not found")
		return
	}
	reply(w, 200, map[string]string{"email": invite.Email, "eventTitle": e.Title, "kind": e.Kind, "studioUrl": studioFor(e.Kind)})
}

func (s *server) acceptCouple(w http.ResponseWriter, r *http.Request) {
	if !accessTokenPattern.MatchString(r.PathValue("token")) {
		bad(w, 404, "Invitation not found")
		return
	}
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
		bad(w, 403, "Sign in with the invited email; check available collaborator slots")
		return
	}
	reply(w, 200, map[string]string{"status": "accepted", "eventId": e.ID})
}

func (s *server) createCoupleAccount(w http.ResponseWriter, r *http.Request) {
	bad(w, 410, "Invite collaborators by email; each person creates their own account")
}
func (s *server) couples() application.Couples {
	return application.Couples{Repository: mongostore.Couples{DB: s.db}, Now: func() time.Time { return time.Now().UTC() }}
}
