package httpapi

import (
	"bytes"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"image"
	"image/png"
	"io"
	"net/http"
	"time"
)

func (s *server) updateProfile(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if decode(r, &in) != nil {
		bad(w, 400, "Invalid profile")
		return
	}
	name, ok := core.ProfileName(in.Name)
	if !ok {
		bad(w, 400, "Use a name between 2 and 80 characters")
		return
	}
	if _, err = s.db.Collection("users").UpdateByID(r.Context(), u.ID, bson.M{"$set": bson.M{"name": name}}); err != nil {
		bad(w, 500, "Could not save profile")
		return
	}
	u.Name = name
	reply(w, 200, u)
}

func avatarImage(raw []byte) ([]byte, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 512 || config.Height > 512 {
		return nil, io.ErrUnexpectedEOF
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err = png.Encode(&output, img); err != nil {
		return nil, err
	}
	if output.Len() > 1<<20 {
		return nil, io.ErrUnexpectedEOF
	}
	return output.Bytes(), nil
}

func (s *server) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	file, _, err := r.FormFile("photo")
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		bad(w, 400, "Choose a photo smaller than 2 MB")
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		bad(w, 400, "Photo is too large")
		return
	}
	encoded, err := avatarImage(raw)
	if err != nil {
		bad(w, 400, "Use a JPEG, PNG or WebP photo, up to 512 pixels")
		return
	}
	version := time.Now().UnixNano()
	_, err = s.db.Collection("profilePhotos").UpdateByID(r.Context(), u.ID, bson.M{"$set": bson.M{"photo": encoded}}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		bad(w, 500, "Could not save photo")
		return
	}
	if _, err = s.db.Collection("users").UpdateByID(r.Context(), u.ID, bson.M{"$set": bson.M{"avatarVersion": version}}); err != nil {
		bad(w, 500, "Could not update profile")
		return
	}
	u.AvatarVersion = version
	reply(w, 200, u)
}

func (s *server) profileAvatar(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	var stored struct {
		Photo []byte `bson:"photo"`
	}
	if s.db.Collection("profilePhotos").FindOne(r.Context(), bson.M{"_id": u.ID}).Decode(&stored) != nil {
		bad(w, 404, "Photo not found")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(stored.Photo)
}
