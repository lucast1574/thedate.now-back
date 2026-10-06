package core

import "time"

const CommissionBPS int64 = 1000
const MinimumWithdrawalCents int64 = 5000

type Payment struct {
	SessionID     string    `bson:"sessionId" json:"sessionId"`
	IntentID      string    `bson:"intentId" json:"-"`
	AmountCents   int64     `bson:"amountCents" json:"amountCents"`
	RefundedCents int64     `bson:"refundedCents" json:"refundedCents"`
	Currency      string    `bson:"currency" json:"currency"`
	Live          bool      `bson:"live" json:"live"`
	PaidAt        time.Time `bson:"paidAt" json:"paidAt"`
}
type Conversion struct {
	ReferrerID      string    `bson:"referrerId" json:"-"`
	EventID         string    `bson:"eventId" json:"eventId"`
	SessionID       string    `bson:"sessionId" json:"-"`
	PaymentCents    int64     `bson:"paymentCents" json:"paymentCents"`
	RefundedCents   int64     `bson:"refundedCents" json:"refundedCents"`
	CommissionCents int64     `bson:"commissionCents" json:"commissionCents"`
	Live            bool      `bson:"live" json:"live"`
	CreatedAt       time.Time `bson:"createdAt" json:"createdAt"`
}
type Withdrawal struct {
	ID          string    `bson:"id" json:"id"`
	AmountCents int64     `bson:"amountCents" json:"amountCents"`
	Method      string    `bson:"method" json:"method"`
	Details     string    `bson:"details" json:"details"`
	Status      string    `bson:"status" json:"status"`
	Note        string    `bson:"note" json:"note"`
	CreatedAt   time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time `bson:"updatedAt" json:"updatedAt"`
}

func AffiliateCommission(amount, refunded int64) int64 {
	if amount <= 0 || refunded >= amount {
		return 0
	}
	if refunded < 0 {
		refunded = 0
	}
	return ((amount-refunded)*CommissionBPS + 5000) / 10000
}
func WithdrawalTransition(from, to string) bool {
	return (from == "pending" && (to == "approved" || to == "rejected")) || (from == "approved" && (to == "paid" || to == "rejected"))
}
func ValidRole(role string) bool {
	switch role {
	case "admin", "planner", "organizer", "couple":
		return true
	}
	return false
}
