package core

import (
	"testing"
	"time"
)

func TestAdmissionIsOptionalAndGeneralOnly(t *testing.T) {
	a := Admission{Enabled: true, Currency: "PEN", AmountCents: 8000, Basis: "person", Instructions: "Yape al organizador"}
	if !ValidAdmission("general", &a) || ValidAdmission("wedding", &a) || !ValidAdmission("wedding", nil) {
		t.Fatal("product boundary")
	}
	if AdmissionTotal(&a, 2) != 16000 {
		t.Fatal("party total")
	}
	a.Basis = "group"
	if AdmissionTotal(&a, 2) != 8000 {
		t.Fatal("group total")
	}
	for _, currency := range []string{"", "BTC", "pen"} {
		a.Currency = currency
		if ValidAdmission("general", &a) {
			t.Fatalf("invalid currency %q", currency)
		}
	}
}
func TestGuestPaymentRequiresValidMoneyDateAndMethod(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	a := Admission{Enabled: true, Currency: "PEN", AmountCents: 8000, Basis: "person", Instructions: "Transferencia"}
	p := GuestPayment{ID: "request-1", AmountCents: 4000, Currency: "PEN", PaidOn: "2026-11-15", Method: "transfer"}
	if !ValidGuestPayment(p, &a, now) {
		t.Fatal("valid payment")
	}
	for _, change := range []func(*GuestPayment){func(v *GuestPayment) { v.AmountCents = 0 }, func(v *GuestPayment) { v.Currency = "USD" }, func(v *GuestPayment) { v.PaidOn = "2026-02-30" }, func(v *GuestPayment) { v.PaidOn = "2028-01-01" }, func(v *GuestPayment) { v.ID = "a.b" }} {
		invalid := p
		change(&invalid)
		if ValidGuestPayment(invalid, &a, now) {
			t.Fatal("invalid payment accepted")
		}
	}
}
