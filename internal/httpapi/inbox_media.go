package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/minio/minio-go/v7"
	"go.mongodb.org/mongo-driver/v2/bson"
	"image"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func safeMessageMediaURL(base, raw string) bool {
	b, err := url.Parse(base)
	if err != nil {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && b.Scheme == "https" && u.Host == b.Host && u.User == nil && strings.HasPrefix(u.Path, "/api/files/") && path.Clean(u.Path) == u.Path
}
func (s *server) ensureMessageMedia(ctx context.Context, m core.InboxMessage) error {
	if m.Media == nil || m.Media.Available || m.Media.URL == "" || s.storage == nil {
		return nil
	}
	config := wazendFor(m.Media.BucketKind)
	if !safeMessageMediaURL(config.Base, m.Media.URL) {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Media.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", config.Key)
	client := *wazendClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("media unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (10<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 10<<20 {
		return nil
	}
	mime := http.DetectContentType(data)
	allowed := strings.HasPrefix(mime, "image/") || strings.HasPrefix(mime, "audio/") || strings.HasPrefix(mime, "video/") || mime == "application/pdf" || mime == "application/ogg"
	if !allowed || mime == "image/svg+xml" {
		return nil
	}
	if strings.HasPrefix(mime, "image/") {
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 40000000 {
			return nil
		}
	}
	key := "messages/" + m.ID
	_, err = s.storage.client.PutObject(ctx, s.storage.bucket(m.Media.BucketKind), key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: mime})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	_, err = s.db.Collection("inboxMessages").UpdateOne(ctx, bson.M{"_id": m.ID}, bson.M{"$set": bson.M{"media.key": key, "media.available": true, "media.mime": mime, "media.digest": hex.EncodeToString(digest[:])}})
	return err
}
func (s *server) inboxMedia(w http.ResponseWriter, r *http.Request) {
	e, err := s.inboxEvent(r)
	if err != nil || e.IsDemo || e.PaymentStatus != "paid" || s.storage == nil {
		bad(w, 404, "Adjunto no encontrado")
		return
	}
	m, err := (mongostore.Inbox{DB: s.db}).Message(r.Context(), r.PathValue("messageId"))
	if err != nil || m.EventID != e.ID || m.Media == nil || !m.Media.Available {
		bad(w, 404, "Adjunto no disponible")
		return
	}
	object, err := s.storage.client.GetObject(r.Context(), s.storage.bucket(m.Media.BucketKind), m.Media.Key, minio.GetObjectOptions{})
	if err != nil {
		bad(w, 502, "No pudimos abrir el adjunto.")
		return
	}
	defer object.Close()
	if _, err = object.Stat(); err != nil {
		bad(w, 404, "Adjunto no disponible")
		return
	}
	w.Header().Set("Content-Type", m.Media.MIME)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline; filename=\"adjunto\"")
	io.Copy(w, io.LimitReader(object, 10<<20))
}
