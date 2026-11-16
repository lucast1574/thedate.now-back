package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"net/http"
	"time"
)

func (s *server) guestPayments(w http.ResponseWriter, r *http.Request) {
	e, err := s.managedEvent(r)
	if err != nil || e.Kind != "general" || e.IsDemo || e.PaymentStatus != "paid" || e.Admission == nil || !e.Admission.Enabled {
		bad(w, 404, "Pagos no disponibles para este evento.")
		return
	}
	if r.Method == http.MethodGet {
		list := []core.GuestPayment{}
		for _, p := range e.GuestPayments {
			list = append(list, p)
		}
		reply(w, 200, list)
		return
	}
	var in struct {
		ID          string `json:"id"`
		GuestID     string `json:"guestId"`
		MessageID   string `json:"messageId"`
		AmountCents int64  `json:"amountCents"`
		Currency    string `json:"currency"`
		PaidOn      string `json:"paidOn"`
		Method      string `json:"method"`
		Reference   string `json:"reference"`
		Verified    bool   `json:"verified"`
	}
	if decode(r, &in) != nil || !in.Verified {
		bad(w, 400, "Verifica el ingreso antes de registrar el pago.")
		return
	}
	u, _ := s.user(r)
	now := time.Now().UTC()
	p := core.GuestPayment{ID: in.ID, GuestID: in.GuestID, MessageID: in.MessageID, AmountCents: in.AmountCents, Currency: in.Currency, PaidOn: in.PaidOn, Method: in.Method, Reference: in.Reference, RecordedBy: u.ID, CreatedAt: now}
	if !core.ValidGuestPayment(p, e.Admission, now) {
		bad(w, 400, "Revisa el importe, la moneda, la fecha y el medio de pago.")
		return
	}
	// Snapshot initializes the shared version for legacy events before the conditional payment write.
	e, err = (mongostore.Responses{DB: s.db}).Snapshot(r.Context(), e.ID)
	if err == nil {
		err = mongostore.RecordGuestPayment(r.Context(), s.db, e, p, now)
	}
	if err != nil {
		bad(w, 409, "No se registró otro abono: el comprobante ya está registrado o hubo cambios. Recarga los pagos antes de continuar.")
		return
	}
	reply(w, 201, p)
}
