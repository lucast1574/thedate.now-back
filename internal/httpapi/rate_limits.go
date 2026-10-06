package httpapi

import (
	"bytes"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func (s *server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || (!strings.HasPrefix(r.URL.Path, "/auth/") && !strings.HasPrefix(r.URL.Path, "/public/rsvp/")) {
			next.ServeHTTP(w, r)
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		key, limit := "ip:"+ip, 300
		allowed, err := mongostore.Allow(r.Context(), s.db, key, limit, time.Now().UTC())
		if err != nil {
			bad(w, 503, "Request protection unavailable")
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", "60")
			bad(w, 429, "Too many attempts; try again in a minute")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/auth/") && r.URL.Path != "/auth/google" {
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if err != nil {
				bad(w, 400, "Invalid request")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			var in credentials
			if json.Unmarshal(body, &in) == nil && validEmail(strings.ToLower(strings.TrimSpace(in.Email))) {
				key = "auth:" + strings.ToLower(strings.TrimSpace(in.Email))
				limit = 10
			}
		}
		allowed, err = mongostore.Allow(r.Context(), s.db, key, limit, time.Now().UTC())
		if err != nil {
			bad(w, 503, "Authentication protection unavailable")
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", "60")
			bad(w, 429, "Too many attempts; try again in a minute")
			return
		}
		next.ServeHTTP(w, r)
	})
}
