package httpapi

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
)

type credentials struct {
	ReferralCode string `json:"referralCode"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	Role         string `json:"role"`
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid request")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	if !validEmail(in.Email) || in.Email == adminEmail() || len(in.Password) < 10 || len(in.Password) > 72 || len(in.Name) < 2 || len(in.Name) > 120 || (in.Role != "planner" && in.Role != "organizer") {
		bad(w, 400, "Name, email, role and password of at least 10 characters required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		bad(w, 500, "Could not create account")
		return
	}
	u := core.User{AffiliateEnabled: true, AffiliateCode: mongostore.AffiliateCode(), ReferredBy: s.referredBy(r, in.ReferralCode), ID: uuid.NewString(), Email: in.Email, Name: in.Name, PasswordHash: string(hash), Role: in.Role, CreatedAt: time.Now().UTC()}
	if _, err = s.db.Collection("users").InsertOne(r.Context(), u); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			bad(w, 409, "Email already registered")
		} else {
			bad(w, 500, "Could not create account")
		}
		return
	}
	token, err := s.sign(u, "password")
	if err != nil {
		bad(w, 500, "Could not sign in")
		return
	}
	reply(w, 201, map[string]any{"user": u, "token": token})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid request")
		return
	}
	var u core.User
	err := s.db.Collection("users").FindOne(r.Context(), bson.M{"email": strings.ToLower(strings.TrimSpace(in.Email))}).Decode(&u)
	if err != nil || u.Email == adminEmail() || u.GoogleSub != "" || len(in.Password) > 72 || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		bad(w, 401, "Invalid credentials")
		return
	}
	if u.Role != "admin" {
		if _, err := (mongostore.Affiliates{DB: s.db}).EnableDefaults(r.Context(), u.ID); err != nil {
			bad(w, 500, "Could not prepare affiliate links")
			return
		}
	}
	token, err := s.sign(u, "password")
	if err != nil {
		bad(w, 500, "Could not sign in")
		return
	}
	reply(w, 200, map[string]any{"user": u, "token": token})
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	u.CreatorPortals = core.UserPortals(u)
	u.Portals = core.UserPortals(u)
	reply(w, 200, u)
}

type sessionClaims struct {
	jwt.RegisteredClaims
	Version int    `json:"version"`
	Format  int    `json:"format"`
	Method  string `json:"method"`
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(email) <= 254
}
func adminEmail() string { return strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))) }
func (s *server) sign(u core.User, method string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, sessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: u.ID, Issuer: "thedate-api", Audience: jwt.ClaimStrings{"thedate-studio"}, ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)), IssuedAt: jwt.NewNumericDate(now)},
		Version:          u.TokenVersion, Format: 1, Method: method,
	}).SignedString(s.secret)
}
func validSession(u core.User, claims *sessionClaims) bool {
	if claims.Format != 1 || claims.Subject != u.ID || claims.Version != u.TokenVersion {
		return false
	}
	return core.SessionMethodAllowed(u, claims.Method, adminEmail(), os.Getenv("GOOGLE_ADMIN_SUB"))
}

func (s *server) user(r *http.Request) (core.User, error) {
	var u core.User
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || raw == "" {
		return u, errors.New("missing token")
	}
	claims := &sessionClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) { return s.secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("thedate-api"), jwt.WithAudience("thedate-studio"), jwt.WithExpirationRequired())
	if err != nil || !token.Valid || claims.Subject == "" {
		return u, errors.New("invalid token")
	}
	err = s.db.Collection("users").FindOne(r.Context(), bson.M{"_id": claims.Subject}).Decode(&u)
	if err != nil {
		return u, err
	}
	if !validSession(u, claims) {
		return core.User{}, errors.New("session revoked")
	}
	return u, nil
}
