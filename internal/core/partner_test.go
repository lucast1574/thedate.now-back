package core

import "testing"

func TestProfessionalQuotesHaveOneBenefitPerEvent(t *testing.T) {
	for _, kind := range []string{"wedding", "general"} {
		role := "planner"
		public, benefit := int64(2000), int64(500)
		if kind == "general" {
			role = "organizer"
			public, benefit = 1000, 200
		}
		owner := User{ID: "pro", Role: role}
		self := QuoteEvent(kind, owner)
		if self.Amount != public || self.Benefit != benefit || self.PartnerID != "pro" {
			t.Fatalf("professional purchase: %+v", self)
		}
		client := QuoteEvent(kind, owner)
		if client != self {
			t.Fatalf("client: %+v", client)
		}
		p := Payment{AmountCents: public, Live: true}
		if PartnerCommission(client, p) != benefit {
			t.Fatal("commission")
		}
		p.RefundedCents = public / 2
		if PartnerCommission(client, p) != benefit/2 {
			t.Fatal("partial refund")
		}
		p.Live = false
		if PartnerCommission(client, p) != 0 {
			t.Fatal("test commission")
		}
		ref := QuoteEvent(kind, User{ID: "client", Role: "user"})
		if ref.PartnerID != "" || ref.Benefit != 0 || ref.Amount != public || ref.Mode != "direct" {
			t.Fatal("normal buyer must not generate professional or referral commission")
		}
	}
}
