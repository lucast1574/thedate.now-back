package httpapi

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/minio/minio-go/v7"
	"go.mongodb.org/mongo-driver/v2/bson"
	"io"
	"net/http"
	"regexp"
	"slices"
	"time"
)

var audioKey = regexp.MustCompile(`^[a-f0-9-]{36}\.mp3$`)

func (s *server) uploadAudio(w http.ResponseWriter, r *http.Request) {
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if s.storage == nil {
		bad(w, 503, "Audio storage unavailable")
		return
	}
	if len(e.AudioKeys) >= 20 {
		bad(w, 409, "Maximum 20 audio uploads per invitation")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		bad(w, 413, "Choose an MP3 smaller than 10 MB")
		return
	}
	if !core.ValidMP3(data) {
		bad(w, 400, "Choose a valid MP3 file")
		return
	}
	key := uuid.NewString() + ".mp3"
	_, err = s.storage.client.PutObject(r.Context(), s.storage.bucket(e.Kind), key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "audio/mpeg"})
	if err != nil {
		bad(w, 502, "Could not upload audio")
		return
	}
	attached, err := mongostore.AttachAudio(r.Context(), s.db, e.ID, key, time.Now().UTC())
	if err != nil || !attached {
		_ = s.storage.client.RemoveObject(r.Context(), s.storage.bucket(e.Kind), key, minio.RemoveObjectOptions{})
		bad(w, 409, "Could not attach audio")
		return
	}
	reply(w, 201, map[string]string{"key": key})
}

func (s *server) ownerAudio(w http.ResponseWriter, r *http.Request) {
	e, err := s.ownedEvent(r)
	key := r.PathValue("key")
	if err != nil || !audioKey.MatchString(key) || !slices.Contains(e.AudioKeys, key) {
		bad(w, 404, "Audio not found")
		return
	}
	s.serveAudio(w, r, e.Kind, key, false)
}

func (s *server) publicAudio(w http.ResponseWriter, r *http.Request) {
	kind, slug, key := r.PathValue("kind"), r.PathValue("slug"), r.PathValue("key")
	if (kind != "wedding" && kind != "general") || core.ValidateSlug(slug) != nil || !audioKey.MatchString(key) {
		bad(w, 404, "Audio not found")
		return
	}
	var e core.Event
	err := s.db.Collection("events").FindOne(r.Context(), bson.M{"kind": kind, "slug": slug, "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}, "music.source": "mp3", "music.value": key}).Decode(&e)
	if err != nil {
		bad(w, 404, "Audio not found")
		return
	}
	s.serveAudio(w, r, kind, key, true)
}

func (s *server) serveAudio(w http.ResponseWriter, r *http.Request, kind, key string, public bool) {
	if s.storage == nil {
		bad(w, 404, "Audio not found")
		return
	}
	object, err := s.storage.client.GetObject(r.Context(), s.storage.bucket(kind), key, minio.GetObjectOptions{})
	if err != nil {
		bad(w, 404, "Audio not found")
		return
	}
	defer object.Close()
	info, err := object.Stat()
	if err != nil {
		bad(w, 404, "Audio not found")
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	if public {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}
	http.ServeContent(w, r, key, info.LastModified, object)
}
