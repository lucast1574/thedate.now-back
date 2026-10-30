package core

import "time"

func InvitationSendState(g Guest, now time.Time) string {
	if g.SendState == "sending" && g.SendStartedAt != nil && now.Sub(*g.SendStartedAt) > 2*time.Minute {
		return "unknown"
	}
	return g.SendState
}

// An unanswered invitation keeps its offered seats until its guest responds.
// Never skip a household merely because a smaller one would fit first.
func CanOfferInvitation(e Event, g Guest, now time.Time) bool {
	if e.RevokedGuestTokens[g.InviteToken] || e.IsDemo || e.PaymentStatus != "paid" || e.PublishedAt == nil || g.EventID != e.ID || g.Seats < 1 {
		return false
	}
	if r, ok := e.Responses[g.ID]; ok && r.Choice != "" && r.Choice != "pending" {
		return false
	}
	if e.InvitationHolds[g.ID] > 0 {
		return true
	}
	return e.CapacityUnlimited || Occupied(e, now)+g.Seats <= e.Capacity
}
