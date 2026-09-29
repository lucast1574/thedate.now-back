package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/api/idtoken"
)

type googleLoginInput struct {
	IDToken string `json:"idToken"`
	Portal  string `json:"portal"`
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
	payload, err := idtoken.Validate(r.Context(), in.IDToken, clientID)
	if err != nil || payload.Subject == "" {
		bad(w, 401, "Invalid Google identity")
		return
	}
	email, _ := payload.Claims["email"].(string)
	verified, _ := payload.Claims["email_verified"].(bool)
	name, _ := payload.Claims["name"].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if !verified || !strings.Contains(email, "@") {
		bad(w, 403, "A verified Google email is required")
		return
	}
	if name == "" {
		name = email
	}
	adminEmail := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	role := "organizer"
	if in.Portal == "wedding" {
		role = "planner"
	}
	if adminEmail != "" && email == adminEmail {
		role = "admin"
	}
	var u core.User
	err = s.db.Collection("users").FindOne(r.Context(), bson.M{"googleSub": payload.Subject}).Decode(&u)
	if err == mongo.ErrNoDocuments {
		err = s.db.Collection("users").FindOne(r.Context(), bson.M{"email": email}).Decode(&u)
		if err == mongo.ErrNoDocuments {
			u = core.User{ID: uuid.NewString(), Email: email, Name: name, Role: role, GoogleSub: payload.Subject, CreatedAt: time.Now().UTC()}
			if _, err = s.db.Collection("users").InsertOne(r.Context(), u); err != nil {
				bad(w, 409, "Could not create Google account")
				return
			}
		} else if err == nil {
			if u.GoogleSub != "" && u.GoogleSub != payload.Subject {
				bad(w, 409, "This email is linked to another Google account")
				return
			}
			changes := bson.M{"googleSub": payload.Subject}
			if role == "admin" {
				changes["role"] = "admin"
				u.Role = "admin"
			}
			if _, err = s.db.Collection("users").UpdateByID(r.Context(), u.ID, bson.M{"$set": changes}); err != nil {
				bad(w, 500, "Could not link Google account")
				return
			}
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
	if role == "admin" && u.Role != "admin" {
		if _, err = s.db.Collection("users").UpdateByID(r.Context(), u.ID, bson.M{"$set": bson.M{"role": "admin"}}); err != nil {
			bad(w, 500, "Could not grant admin access")
			return
		}
		u.Role = "admin"
	}
	token, err := s.sign(u.ID)
	if err != nil {
		bad(w, 500, "Could not sign in")
		return
	}
	reply(w, 200, map[string]any{"user": u, "token": token})
}
