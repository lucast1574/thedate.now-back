package core

import "time"

const PartnerHold = 7 * 24 * time.Hour

type CheckoutQuote struct {
	Amount    int64  `bson:"amount" json:"amount"`
	PartnerID string `bson:"partnerId,omitempty" json:"partnerId,omitempty"`
	Benefit   int64  `bson:"benefit" json:"benefit"`
	Mode      string `bson:"mode" json:"mode"`
}
type PartnerEntry struct {
	EventID     string    `bson:"eventId" json:"eventId"`
	Kind        string    `bson:"kind" json:"kind"`
	Title       string    `bson:"title" json:"title"`
	Amount      int64     `bson:"amount" json:"amount"`
	Credited    int64     `bson:"credited" json:"credited"`
	AvailableAt time.Time `bson:"availableAt" json:"availableAt"`
}

func ProfessionalFor(kind string, u User) bool {
	return u.Role == "planner" && kind == "wedding" || u.Role == "organizer" && kind == "general"
}
func PartnerBenefit(kind string) int64 {
	if kind == "wedding" {
		return 500
	}
	return 200
}
func QuoteEvent(kind string, owner User, referrer User, mode string) CheckoutQuote {
	p, _ := ProductFor(kind)
	q := CheckoutQuote{Amount: p.PriceCents, Mode: "public"}
	if ProfessionalFor(kind, owner) {
		q.Mode = "self"
		q.Amount -= PartnerBenefit(kind)
		if mode == "client" {
			q.Mode = "client"
			q.Amount = p.PriceCents
			q.PartnerID = owner.ID
			q.Benefit = PartnerBenefit(kind)
		}
	} else if ProfessionalFor(kind, referrer) && referrer.ID != owner.ID {
		q.PartnerID = referrer.ID
		q.Benefit = PartnerBenefit(kind)
	}
	return q
}
func PartnerCommission(q CheckoutQuote, p Payment) int64 {
	if !p.Live || q.Benefit <= 0 || p.AmountCents <= 0 || p.RefundedCents >= p.AmountCents {
		return 0
	}
	return q.Benefit * (p.AmountCents - p.RefundedCents) / p.AmountCents
}
