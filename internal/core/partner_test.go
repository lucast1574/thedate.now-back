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
		self := QuoteEvent(kind, owner, User{ID: "other", Role: role}, "")
		if self.Amount != public-benefit || self.Benefit != 0 || self.PartnerID != "" {
			t.Fatalf("discount: %+v", self)
		}
		client := QuoteEvent(kind, owner, User{}, "client")
		if client.Amount != public || client.PartnerID != "pro" || client.Benefit != benefit {
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
		ref := QuoteEvent(kind, User{ID: "client", Role: "user"}, owner, "")
		if ref.PartnerID != "pro" || ref.Amount != public {
			t.Fatal("referral")
		}
	}
}
