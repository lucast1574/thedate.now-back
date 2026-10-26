package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"net/http"
	"time"
)

func (s *server) giftRequestLimit(w http.ResponseWriter, r *http.Request, key string, limit int) bool {
	allowed, err := mongostore.Allow(r.Context(), s.db, key, limit, time.Now().UTC())
	if err != nil {
		bad(w, 503, "No podemos procesar esta solicitud ahora.")
		return false
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		bad(w, 429, "Espera un momento antes de volver a intentarlo.")
		return false
	}
	return true
}
