package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"net/http"
	"time"
)

func (s *server) payWithBalance(w http.ResponseWriter, r *http.Request, e core.Event, u core.User, q core.CheckoutQuote) {
	repo := mongostore.Affiliates{DB: s.db}
	if _, err := repo.PartnerEntries(r.Context(), u.ID, time.Now().UTC()); err != nil {
		bad(w, 500, "Could not release credit")
		return
	}
	if err := repo.PayBalance(r.Context(), e, u, q.Amount, time.Now().UTC()); err != nil {
		bad(w, 409, err.Error())
		return
	}
	reply(w, 200, map[string]string{"url": "/editor/" + e.ID})
}
func (s *server) cancelBalance(w http.ResponseWriter, r *http.Request, e core.Event, u core.User) {
	if err := (mongostore.Affiliates{DB: s.db}).CancelBalance(r.Context(), e, u); err != nil {
		bad(w, 409, err.Error())
		return
	}
	reply(w, 200, map[string]string{"url": "/?event=" + e.ID})
}
