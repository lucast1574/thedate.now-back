package httpapi

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/api/idtoken"
)

type googleLoginInput struct {
	ReferralCode string `json:"referralCode"`
	IDToken      string `json:"idToken"`
	Portal       string `json:"portal"`
}

func (s *server) googleLogin(w http.ResponseWriter, r *http.Request) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		bad(w, 503, "Google sign-in is not configured")
		return
	}
	var in googleLoginInput
	if decode(r, &in) != nil || len(in.IDToken) > 10000 || (in.Portal != "wedding" && in.Portal != "general") {
		bad(w, 400, "Invalid Google sign-in")
		return
	}
	verify := s.verifyGoogle
	if verify == nil {
		verify = idtoken.Validate
	}
	payload, err := verify(r.Context(), in.IDToken, clientID)
	if err != nil || payload.Subject == "" {
		bad(w, 401, "Invalid Google identity")
		return
	}
	email, _ := payload.Claims["email"].(string)
	verified, _ := payload.Claims["email_verified"].(bool)
	name, _ := payload.Claims["name"].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if !verified || !validEmail(email) {
		bad(w, 403, "A verified Google email is required")
		return
	}
	if name == "" {
		name = email
	}
	hd, _ := payload.Claims["hd"].(string)
	authoritative := strings.HasSuffix(email, "@gmail.com") || hd != ""
	adminEmail := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	role := "organizer"
	if in.Portal == "wedding" {
		role = "planner"
	}
	if adminEmail != "" && email == adminEmail {
		if !authoritative && payload.Subject != os.Getenv("GOOGLE_ADMIN_SUB") {
			bad(w, 403, "Admin Google identity requires an authoritative email or pinned subject")
			return
		}
		role = "admin"
	}
	var u core.User
	err = s.db.Collection("users").FindOne(r.Context(), bson.M{"googleSub": payload.Subject}).Decode(&u)
	if err == mongo.ErrNoDocuments {
		err = s.db.Collection("users").FindOne(r.Context(), bson.M{"email": email}).Decode(&u)
		if err == mongo.ErrNoDocuments {
			u = core.User{ReferredBy: s.referredBy(r, in.ReferralCode), ID: uuid.NewString(), Email: email, Name: name, Role: role, GoogleAuthoritative: authoritative, IdentityPolicy: 1, GoogleSub: payload.Subject, CreatedAt: time.Now().UTC()}
			if _, err = s.db.Collection("users").InsertOne(r.Context(), u); err != nil {
				bad(w, 409, "Could not create Google account")
				return
			}
		} else if err == nil {
			if !authoritative && !(role == "admin" && payload.Subject == os.Getenv("GOOGLE_ADMIN_SUB")) {
				bad(w, 409, "This email requires explicit account verification before linking Google")
				return
			}
			if u.GoogleSub != "" && u.GoogleSub != payload.Subject {
				bad(w, 409, "This email is linked to another Google account")
				return
			}
			// Verified Google ownership replaces unverified local credentials.
			// The conditional write prevents simultaneous identities claiming one account.
			changes := bson.M{"googleSub": payload.Subject, "passwordHash": "", "identityPolicy": 1}
			if role == "admin" {
				changes["role"] = "admin"
				u.Role = "admin"
			}
			result, updateErr := s.db.Collection("users").UpdateOne(r.Context(), bson.M{"_id": u.ID, "googleSub": bson.M{"$in": bson.A{nil, "", payload.Subject}}, "tokenVersion": bson.M{"$in": bson.A{nil, u.TokenVersion}}}, bson.M{"$set": changes, "$inc": bson.M{"tokenVersion": 1}})
			if updateErr != nil || result.MatchedCount != 1 {
				bad(w, 409, "Identity changed; sign in again")
				return
			}
			u.GoogleSub = payload.Subject
			u.PasswordHash = ""
			u.IdentityPolicy = 1
			u.TokenVersion++

		}
	}
	if err != nil && err != mongo.ErrNoDocuments {
		bad(w, 500, "Could not load account")
		return
	}
	if u.Email != email {
		bad(w, 403, "Google identity changed")
		return
	}
	// Invalidate pre-hardening sessions/passwords for existing linked accounts.
	if u.IdentityPolicy != 1 {
		result, updateErr := s.db.Collection("users").UpdateOne(r.Context(), bson.M{"_id": u.ID, "identityPolicy": bson.M{"$ne": 1}}, bson.M{"$set": bson.M{"passwordHash": "", "identityPolicy": 1}, "$inc": bson.M{"tokenVersion": 1}})
		if updateErr != nil || result.MatchedCount != 1 {
			bad(w, 409, "Sign in again")
			return
		}
		u.PasswordHash = ""
		u.IdentityPolicy = 1
		u.TokenVersion++
	}
	if role == "admin" && u.Role != "admin" {
		if _, err = s.db.Collection("users").UpdateByID(r.Context(), u.ID, bson.M{"$set": bson.M{"role": "admin"}}); err != nil {
			bad(w, 500, "Could not grant admin access")
			return
		}
		u.Role = "admin"
	}
	if _, err = s.db.Collection("users").UpdateByID(r.Context(), u.ID, bson.M{"$set": bson.M{"googleAuthoritative": authoritative}}); err != nil {
		bad(w, 500, "Could not update identity")
		return
	}
	u.GoogleAuthoritative = authoritative
	picture, _ := payload.Claims["picture"].(string)
	if picture = core.GooglePhoto(picture); picture != "" {
		if _, err = s.db.Collection("users").UpdateByID(r.Context(), u.ID, bson.M{"$set": bson.M{"googlePhotoUrl": picture}}); err != nil {
			bad(w, 500, "Could not update profile")
			return
		}
		u.GooglePhotoURL = picture
	}
	if u.Role != "couple" && u.Role != "admin" {
		if _, err = s.db.Collection("users").UpdateByID(r.Context(), u.ID, bson.M{"$addToSet": bson.M{"portals": in.Portal}}); err != nil {
			bad(w, 500, "Could not update portal access")
			return
		}
		u.Portals = append(u.Portals, in.Portal)
	}
	if u.Role != "admin" {
		if _, err := (mongostore.Affiliates{DB: s.db}).EnableDefaults(r.Context(), u.ID); err != nil {
			bad(w, 500, "Could not prepare affiliate links")
			return
		}
	}
	token, err := s.sign(u, "google")
	if err != nil {
		bad(w, 500, "Could not sign in")
		return
	}
	reply(w, 200, map[string]any{"user": u, "token": token})
}
