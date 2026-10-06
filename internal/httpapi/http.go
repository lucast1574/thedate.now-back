package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

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
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one JSON object required")
	}
	return nil
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allow := map[string]bool{"https://thedate.now": true, "https://save.thedate.now": true, "https://backoffice.thedate.now": true, "https://crea.thedate.now": true, "https://studio.save.thedate.now": true}
		origin := r.Header.Get("Origin")
		if origin != "" && !allow[origin] {
			bad(w, 403, "Origin is not allowed")
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
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
