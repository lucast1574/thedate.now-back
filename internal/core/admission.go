package core

import (
	"errors"
	"strings"
	"time"
)

type Admission struct {
	Enabled      bool   `bson:"enabled" json:"enabled"`
	Currency     string `bson:"currency" json:"currency"`
	AmountCents  int64  `bson:"amountCents" json:"amountCents"`
	Basis        string `bson:"basis" json:"basis"`
	Instructions string `bson:"instructions" json:"instructions"`
}

var ErrAdmissionLocked = errors.New("No puedes cambiar el costo o la moneda después de registrar pagos.")

func ValidAdmission(kind string, a *Admission) bool {
	if a == nil {
		return true
	}
	if kind != "general" {
		return false
	}
	if !a.Enabled {
		return a.AmountCents == 0 && a.Instructions == ""
	}
	return (a.Currency == "PEN" || a.Currency == "USD" || a.Currency == "MXN" || a.Currency == "COP" || a.Currency == "EUR" || a.Currency == "CLP" || a.Currency == "ARS") && a.AmountCents > 0 && a.AmountCents <= 100000000 && (a.Basis == "person" || a.Basis == "group") && len(strings.TrimSpace(a.Instructions)) > 0 && len(a.Instructions) <= 2000
}

func SameAdmissionCost(a, b *Admission) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Enabled == b.Enabled && a.Currency == b.Currency && a.AmountCents == b.AmountCents && a.Basis == b.Basis
}

func AdmissionTotal(a *Admission, seats int) int64 {
	if a == nil || !a.Enabled {
		return 0
	}
	if a.Basis == "group" {
		return a.AmountCents
	}
	return a.AmountCents * int64(max(1, seats))
}

func GuestPaymentBalance(e Event, g Guest) int64 {
	paid := int64(0)
	for _, payment := range e.GuestPayments {
		if payment.GuestID == g.ID {
			paid += payment.AmountCents
		}
	}
	seats := g.Seats
	if response := e.Responses[g.ID]; response.PartyRegistered {
		seats = response.Seats
	}
	return max(0, AdmissionTotal(e.Admission, seats)-paid)
}

type GuestPayment struct {
	ReceiptDigest string    `bson:"receiptDigest,omitempty" json:"-"`
	ID            string    `bson:"id" json:"id"`
	GuestID       string    `bson:"guestId" json:"guestId"`
	MessageID     string    `bson:"messageId,omitempty" json:"messageId,omitempty"`
	AmountCents   int64     `bson:"amountCents" json:"amountCents"`
	Currency      string    `bson:"currency" json:"currency"`
	PaidOn        string    `bson:"paidOn" json:"paidOn"`
	Method        string    `bson:"method" json:"method"`
	Reference     string    `bson:"reference" json:"reference"`
	RecordedBy    string    `bson:"recordedBy" json:"recordedBy"`
	CreatedAt     time.Time `bson:"createdAt" json:"createdAt"`
}

func ValidGuestPayment(p GuestPayment, a *Admission, now time.Time) bool {
	date, err := time.Parse("2006-01-02", p.PaidOn)
	return a != nil && a.Enabled && p.Currency == a.Currency && p.AmountCents > 0 && p.AmountCents <= 100000000 && err == nil && !date.After(now.Add(24*time.Hour)) && (p.Method == "transfer" || p.Method == "wallet" || p.Method == "cash") && len(p.Reference) <= 500 && p.ID != "" && len(p.ID) <= 64 && !strings.ContainsAny(p.ID, ".$")
}
