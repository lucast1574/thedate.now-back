package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type server struct {
	db      *mongo.Database
	secret  []byte
	storage *storage
	locks   sync.Map
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	uri := os.Getenv("MONGODB_URI")
	secret := os.Getenv("JWT_SECRET")
	if uri == "" || len(secret) < 32 {
		log.Fatal("MONGODB_URI and JWT_SECRET (at least 32 bytes) are required")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}
	dbName := env("MONGODB_DATABASE", "thedate")
	s := &server{db: client.Database(dbName), secret: []byte(secret)}
	s.storage, err = newStorage(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := s.indexes(ctx); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("POST /auth/register", s.register)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("GET /auth/me", s.me)
	mux.HandleFunc("GET /events", s.listEvents)
	mux.HandleFunc("POST /events", s.createEvent)
	mux.HandleFunc("PATCH /events/{id}", s.updateEvent)
	mux.HandleFunc("GET /events/{id}/guests", s.listGuests)
	mux.HandleFunc("POST /events/{id}/guests", s.addGuest)
	mux.HandleFunc("POST /events/{id}/photos", s.uploadPhoto)
	mux.HandleFunc("GET /events/{id}/photos/{key}", s.ownerPhoto)
	mux.HandleFunc("POST /events/{id}/send-invitations", s.sendInvitations)
	mux.HandleFunc("POST /events/{id}/couple-invitations", s.inviteCouple)
	mux.HandleFunc("GET /couple-invites/{token}", s.coupleInviteDetails)
	mux.HandleFunc("POST /couple-invites/{token}/accept", s.acceptCouple)
	mux.HandleFunc("GET /public/events/{kind}/{slug}", s.publicEvent)
	mux.HandleFunc("GET /public/events/{kind}/{slug}/photos/{key}", s.publicPhoto)
	mux.HandleFunc("POST /public/rsvp/{token}", s.rsvp)
	mux.HandleFunc("GET /public/rsvp/{token}", s.rsvpDetails)
	mux.HandleFunc("POST /events/{id}/checkout", s.checkout)
	mux.HandleFunc("POST /events/{id}/publish", s.publish)
	mux.HandleFunc("POST /webhooks/stripe", s.stripeWebhook)
	mux.HandleFunc("POST /webhooks/wazend", s.wazendWebhook)
	port := env("PORT", "8080")
	log.Printf("The Date API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, cors(mux)))
}

func env(k, fallback string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return fallback
}
func reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func bad(w http.ResponseWriter, status int, message string) {
	reply(w, status, map[string]string{"error": message})
}
func decode(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	return d.Decode(dst)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allow := map[string]bool{"https://thedate.now": true, "https://save.thedate.now": true, "https://backoffice.thedate.now": true, "https://crea.thedate.now": true, "https://studio.save.thedate.now": true}
		origin := r.Header.Get("Origin")
		if allow[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) indexes(ctx context.Context) error {
	_, err := s.db.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		return err
	}
	_, err = s.db.Collection("events").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		return err
	}
	_, err = s.db.Collection("guests").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "inviteToken", Value: 1}}, Options: options.Index().SetUnique(true)})
	return err
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid request")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	if !strings.Contains(in.Email, "@") || len(in.Password) < 10 || len(in.Name) < 2 || (in.Role != "planner" && in.Role != "organizer" && in.Role != "couple") {
		bad(w, 400, "Name, email, role and password of at least 10 characters required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		bad(w, 500, "Could not create account")
		return
	}
	u := core.User{ID: uuid.NewString(), Email: in.Email, Name: in.Name, PasswordHash: string(hash), Role: in.Role, CreatedAt: time.Now().UTC()}
	if _, err = s.db.Collection("users").InsertOne(r.Context(), u); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			bad(w, 409, "Email already registered")
		} else {
			bad(w, 500, "Could not create account")
		}
		return
	}
	token, err := s.sign(u.ID)
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
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		bad(w, 401, "Invalid credentials")
		return
	}
	token, err := s.sign(u.ID)
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
	reply(w, 200, u)
}
func (s *server) sign(id string) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: id, ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now())}).SignedString(s.secret)
}
func (s *server) user(r *http.Request) (core.User, error) {
	var u core.User
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if raw == "" {
		return u, errors.New("missing token")
	}
	token, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("invalid method")
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return u, errors.New("invalid token")
	}
	id, err := token.Claims.GetSubject()
	if err != nil {
		return u, err
	}
	err = s.db.Collection("users").FindOne(r.Context(), bson.M{"_id": id}).Decode(&u)
	return u, err
}

type eventInput struct {
	Kind           string    `json:"kind"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	StartAt        time.Time `json:"startAt"`
	Location       string    `json:"location"`
	Capacity       int       `json:"capacity"`
	MaybeHoldHours int       `json:"maybeHoldHours"`
	Template       string    `json:"template"`
	AccentColor    string    `json:"accentColor"`
}

func (s *server) createEvent(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	var in eventInput
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid request")
		return
	}
	if (in.Kind != "wedding" && in.Kind != "general") || core.ValidateSlug(in.Slug) != nil || len(strings.TrimSpace(in.Title)) < 2 || in.Capacity < 1 || in.StartAt.IsZero() {
		bad(w, 400, "Invalid event details")
		return
	}
	if in.Kind == "wedding" && u.Role != "planner" {
		bad(w, 403, "Wedding events require a planner account")
		return
	}
	if in.Kind == "general" && u.Role != "organizer" {
		bad(w, 403, "General events require an organizer account")
		return
	}
	if in.MaybeHoldHours == 0 {
		in.MaybeHoldHours = 48
	}
	if in.MaybeHoldHours < 1 || in.MaybeHoldHours > 168 {
		bad(w, 400, "Maybe hold must be between 1 and 168 hours")
		return
	}
	now := time.Now().UTC()
	e := core.Event{ID: uuid.NewString(), Kind: in.Kind, Slug: in.Slug, Title: strings.TrimSpace(in.Title), Description: in.Description, StartAt: in.StartAt, Location: in.Location, Capacity: in.Capacity, MaybeHoldHours: in.MaybeHoldHours, Template: in.Template, AccentColor: in.AccentColor, OwnerID: u.ID, PaymentStatus: "unpaid", CreatedAt: now, UpdatedAt: now, PhotoKeys: []string{}, CoupleUserIDs: []string{}}
	if _, err = s.db.Collection("events").InsertOne(r.Context(), e); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			bad(w, 409, "That invitation address is already taken")
		} else {
			bad(w, 500, "Could not create event")
		}
		return
	}
	reply(w, 201, e)
}
func (s *server) listEvents(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	cur, err := s.db.Collection("events").Find(r.Context(), bson.M{"$or": bson.A{bson.M{"ownerId": u.ID}, bson.M{"coupleUserIds": u.ID}}})
	if err != nil {
		bad(w, 500, "Could not list events")
		return
	}
	defer cur.Close(r.Context())
	events := []core.Event{}
	if err = cur.All(r.Context(), &events); err != nil {
		bad(w, 500, "Could not list events")
		return
	}
	reply(w, 200, events)
}
func (s *server) ownedEvent(r *http.Request) (core.Event, error) {
	var e core.Event
	u, err := s.user(r)
	if err != nil {
		return e, err
	}
	err = s.db.Collection("events").FindOne(r.Context(), bson.M{"_id": r.PathValue("id"), "$or": bson.A{bson.M{"ownerId": u.ID}, bson.M{"coupleUserIds": u.ID}}}).Decode(&e)
	return e, err
}
func (s *server) updateEvent(w http.ResponseWriter, r *http.Request) {
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	var in eventInput
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid request")
		return
	}
	if in.Capacity < 1 || len(strings.TrimSpace(in.Title)) < 2 || in.StartAt.IsZero() {
		bad(w, 400, "Invalid event details")
		return
	}
	hold := in.MaybeHoldHours
	if hold == 0 {
		hold = e.MaybeHoldHours
	}
	if hold < 1 || hold > 168 {
		bad(w, 400, "Invalid hold period")
		return
	}
	changes := bson.M{"title": strings.TrimSpace(in.Title), "description": in.Description, "startAt": in.StartAt, "location": in.Location, "capacity": in.Capacity, "maybeHoldHours": hold, "template": in.Template, "accentColor": in.AccentColor, "updatedAt": time.Now().UTC()}
	_, err = s.db.Collection("events").UpdateByID(r.Context(), e.ID, bson.M{"$set": changes})
	if err != nil {
		bad(w, 500, "Could not update event")
		return
	}
	reply(w, 200, map[string]string{"status": "updated"})
}
func (s *server) publicEvent(w http.ResponseWriter, r *http.Request) {
	kind, slug := r.PathValue("kind"), r.PathValue("slug")
	if (kind != "wedding" && kind != "general") || core.ValidateSlug(slug) != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	var e core.Event
	err := s.db.Collection("events").FindOne(r.Context(), bson.M{"kind": kind, "slug": slug, "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}}).Decode(&e)
	if err != nil {
		bad(w, 404, "Invitation not found")
		return
	}
	reply(w, 200, e)
}
func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type guestInput struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Seats int    `json:"seats"`
}

func (s *server) addGuest(w http.ResponseWriter, r *http.Request) {
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	var in guestInput
	if decode(r, &in) != nil || len(strings.TrimSpace(in.Name)) < 2 || !strings.HasPrefix(in.Phone, "+") || in.Seats < 1 || in.Seats > 20 {
		bad(w, 400, "A name, international phone number and 1-20 seats are required")
		return
	}
	token, err := randomToken()
	if err != nil {
		bad(w, 500, "Could not create invitation")
		return
	}
	g := core.Guest{ID: uuid.NewString(), EventID: e.ID, Name: strings.TrimSpace(in.Name), Phone: in.Phone, Seats: in.Seats, Response: "pending", InviteToken: token, CreatedAt: time.Now().UTC()}
	if _, err = s.db.Collection("guests").InsertOne(r.Context(), g); err != nil {
		bad(w, 500, "Could not add guest")
		return
	}
	reply(w, 201, g)
}
func (s *server) listGuests(w http.ResponseWriter, r *http.Request) {
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
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
	views := make([]map[string]any, 0, len(guests))
	for _, guest := range guests {
		views = append(views, guestView(e, guest))
	}
	reply(w, 200, views)
}

func guestView(e core.Event, g core.Guest) map[string]any {
	host, _ := core.InvitationHost(e.Kind, e.Slug)
	return map[string]any{"id": g.ID, "name": g.Name, "phone": g.Phone, "seats": g.Seats, "response": g.Response, "maybeReason": g.MaybeReason, "maybeExpiresAt": g.MaybeExpiresAt, "sentAt": g.SentAt, "invitationUrl": "https://" + host + "/rsvp/" + g.InviteToken}
}

type rsvpInput struct {
	Response string `json:"response"`
	Reason   string `json:"reason"`
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
	reply(w, 200, map[string]any{"guestName": g.Name, "seats": g.Seats, "response": g.Response, "eventTitle": e.Title, "kind": e.Kind, "slug": e.Slug})
}

func (s *server) rsvp(w http.ResponseWriter, r *http.Request) {
	var in rsvpInput
	if decode(r, &in) != nil || (in.Response != "going" && in.Response != "not_going" && in.Response != "maybe") || (in.Response == "maybe" && len(strings.TrimSpace(in.Reason)) < 3) {
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
	if e.PublishedAt == nil {
		bad(w, 403, "Invitation is not published")
		return
	}
	if err := s.applyResponse(r.Context(), e, g, in.Response, in.Reason); err != nil {
		if errors.Is(err, errCapacity) {
			bad(w, 409, "There are not enough seats available")
		} else {
			bad(w, 500, "Could not save response")
		}
		return
	}
	reply(w, 200, map[string]string{"status": "saved"})
}
