package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Wrap registered handlers so PathValue is populated. Check the stored event,
// never an incoming date: a completed event cannot be reopened by editing it.
func (s *server) eventWriteGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && strings.HasPrefix(r.URL.Path, "/events/") {
			if e, err := s.ownedEvent(r); err == nil && core.EventClosed(e, time.Now().UTC()) {
				bad(w, http.StatusConflict, "Esta acción no está disponible.")
				return
			}
		}
		next(w, r)
	}
}

func (s *server) invitationSendOpen(ctx context.Context, id string) bool {
	var e core.Event
	return s.db.Collection("events").FindOne(ctx, bson.M{"_id": id}).Decode(&e) == nil &&
		!core.EventClosed(e, time.Now().UTC()) && !e.IsDemo && e.PaymentStatus == "paid" && e.PublishedAt != nil
}
