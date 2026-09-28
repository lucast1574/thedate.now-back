package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type storage struct {
	client           *minio.Client
	weddings, events string
}

func newStorage(ctx context.Context) (*storage, error) {
	endpoint, access, secret := os.Getenv("S3_ENDPOINT"), os.Getenv("S3_ACCESS_KEY_ID"), os.Getenv("S3_SECRET_ACCESS_KEY")
	if endpoint == "" && access == "" && secret == "" {
		return nil, nil
	}
	if endpoint == "" || access == "" || secret == "" {
		return nil, errors.New("incomplete S3 configuration")
	}
	client, err := minio.New(endpoint, &minio.Options{Creds: miniocreds.NewStaticV4(access, secret, ""), Secure: os.Getenv("S3_USE_SSL") == "true"})
	if err != nil {
		return nil, err
	}
	s := &storage{client: client, weddings: env("S3_WEDDING_BUCKET", "thedate-weddings"), events: env("S3_EVENT_BUCKET", "thedate-events")}
	for _, bucket := range []string{s.weddings, s.events} {
		exists, err := client.BucketExists(ctx, bucket)
		if err != nil {
			return nil, err
		}
		if !exists {
			if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

func (s *storage) bucket(kind string) string {
	if kind == "wedding" {
		return s.weddings
	}
	return s.events
}

func (s *server) uploadPhoto(w http.ResponseWriter, r *http.Request) {
	if s.storage == nil {
		bad(w, 503, "Photo storage is not configured")
		return
	}
	e, err := s.ownedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	file, _, err := r.FormFile("photo")
	if err != nil {
		bad(w, 400, "Choose an image smaller than 10 MB")
		return
	}
	defer file.Close()
	header := make([]byte, 512)
	count, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		bad(w, 400, "Invalid image")
		return
	}
	mime := http.DetectContentType(header[:count])
	extension := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}[mime]
	if extension == "" {
		bad(w, 400, "Only JPEG, PNG and WebP images are supported")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		bad(w, 400, "Could not read image")
		return
	}
	key := uuid.NewString() + "." + extension
	_, err = s.storage.client.PutObject(r.Context(), s.storage.bucket(e.Kind), key, file, -1, minio.PutObjectOptions{ContentType: mime})
	if err != nil {
		bad(w, 502, "Could not save image")
		return
	}
	_, err = s.db.Collection("events").UpdateByID(r.Context(), e.ID, bson.M{"$push": bson.M{"photoKeys": key}, "$set": bson.M{"updatedAt": time.Now().UTC()}})
	if err != nil {
		_ = s.storage.client.RemoveObject(r.Context(), s.storage.bucket(e.Kind), key, minio.RemoveObjectOptions{})
		bad(w, 500, "Could not attach image")
		return
	}
	reply(w, 201, map[string]string{"key": key})
}

func (s *server) publicPhoto(w http.ResponseWriter, r *http.Request) {
	if s.storage == nil {
		bad(w, 404, "Image not found")
		return
	}
	kind, slug, key := r.PathValue("kind"), r.PathValue("slug"), r.PathValue("key")
	if (kind != "wedding" && kind != "general") || core.ValidateSlug(slug) != nil || strings.Contains(key, "/") {
		bad(w, 404, "Image not found")
		return
	}
	var e core.Event
	filter := bson.M{"kind": kind, "slug": slug, "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}, "photoKeys": key}
	if err := s.db.Collection("events").FindOne(r.Context(), filter).Decode(&e); err != nil {
		bad(w, 404, "Image not found")
		return
	}
	object, err := s.storage.client.GetObject(r.Context(), s.storage.bucket(kind), key, minio.GetObjectOptions{})
	if err != nil {
		bad(w, 404, "Image not found")
		return
	}
	defer object.Close()
	info, err := object.Stat()
	if err != nil {
		bad(w, 404, "Image not found")
		return
	}
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = io.Copy(w, object)
}
