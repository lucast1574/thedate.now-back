package httpapi

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"net/http"
	"strconv"
	"strings"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
)

func supportImages(input []string) ([]core.SupportImage, error) {
	if len(input) > 3 {
		return nil, errors.New("attach up to three screenshots")
	}
	images := make([]core.SupportImage, 0, len(input))
	for _, encoded := range input {
		prefix, raw, ok := strings.Cut(encoded, ",")
		if !ok || (prefix != "data:image/png;base64" && prefix != "data:image/jpeg;base64" && prefix != "data:image/webp;base64") || len(raw) > base64.StdEncoding.EncodedLen(core.MaxSupportImageBytes) {
			return nil, errors.New("invalid screenshot")
		}
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(data) == 0 || len(data) > core.MaxSupportImageBytes {
			return nil, errors.New("screenshots must be smaller than 1 MiB")
		}
		mime := http.DetectContentType(data)
		if prefix != "data:"+mime+";base64" {
			return nil, errors.New("invalid screenshot type")
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 6000 || config.Height > 6000 || int64(config.Width)*int64(config.Height) > 12000000 {
			return nil, errors.New("invalid screenshot dimensions")
		}
		if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
			return nil, errors.New("corrupt screenshot")
		}
		images = append(images, core.SupportImage{MIME: mime, Data: data})
	}
	return images, nil
}
func (s *server) supportImage(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 || index > 2 {
		bad(w, 404, "Screenshot not found")
		return
	}
	owner := u.ID
	if u.Role == "admin" {
		owner = ""
	}
	ticket, err := (mongostore.SupportTickets{DB: s.db}).Get(r.Context(), r.PathValue("id"), owner, true)
	if err != nil || index >= len(ticket.Images) {
		bad(w, 404, "Screenshot not found")
		return
	}
	img := ticket.Images[index]
	w.Header().Set("Content-Type", img.MIME)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Write(img.Data)
}
