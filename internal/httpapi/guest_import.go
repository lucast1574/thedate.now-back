package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"net/http"
	"time"
)

func (s *server) importGuests(w http.ResponseWriter, r *http.Request) {
	e, err := s.planningEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 403, "Pay before importing guests")
		return
	}
	var in struct {
		Guests []guestInput `json:"guests"`
	}
	if decode(r, &in) != nil || len(in.Guests) < 1 || len(in.Guests) > 500 {
		bad(w, 400, "Import 1-500 guests at a time")
		return
	}
	for _, row := range in.Guests {
		if !validGuest(row) {
			bad(w, 400, "Revisa nombre, WhatsApp, máximo un acompañante y categoría de cada fila.")
			return
		}
	}
	added, updated, skipped, duplicates := 0, 0, 0, 0
	seen := map[string]bool{}
	review := []map[string]string{}
	position, err := (mongostore.InvitationQueue{DB: s.db}).Allocate(r.Context(), e.ID, len(in.Guests))
	if err != nil {
		bad(w, 500, "Could not reserve invitation order")
		return
	}
	for index, row := range in.Guests {
		if seen[row.Phone] {
			skipped++
			duplicates++
			continue
		}
		seen[row.Phone] = true
		token, err := randomToken()
		if err != nil {
			bad(w, 500, "Import interrupted; retry the file safely")
			return
		}
		g := guestFromInput(row, core.GuestPhoneID(e.ID, row.Phone), e.ID, token)
		g.CreatedAt = time.Now().UTC()
		g.QueuePosition = position + int64(index)
		result, err := (mongostore.GuestImports{DB: s.db}).Apply(r.Context(), g)
		if err != nil {
			bad(w, 500, "Import interrupted; retry the file safely")
			return
		}
		if result.Added {
			added++
		} else if result.Updated {
			updated++
		} else {
			skipped++
		}
		if result.Review {
			review = append(review, map[string]string{"phone": row.Phone, "name": g.Name, "reason": "Conservamos el +1 y los acompañantes existentes. El Excel propone un cambio que requiere revisión."})
		}
	}
	reply(w, 200, map[string]any{"added": added, "updated": updated, "skipped": skipped, "duplicates": duplicates, "review": review})
}
