package core

import (
	"errors"
	"time"
)

var ErrCoupleAccess = errors.New("invited verified identity required")
var ErrCoupleCapacity = errors.New("two couple accounts or active invitations already exist")

func ActiveCoupleInvites(e Event, now time.Time) map[string]CoupleInvite {
	invites := map[string]CoupleInvite{}
	for key, invite := range e.CoupleInvites {
		if !invite.Accepted && invite.ExpiresAt.After(now) {
			invites[key] = invite
		}
	}
	return invites
}
func AddCoupleInvite(e Event, invite CoupleInvite, now time.Time) (map[string]CoupleInvite, error) {
	if e.Kind != "wedding" || e.PaymentStatus != "paid" || e.IsDemo {
		return nil, ErrUnavailable
	}
	invites := ActiveCoupleInvites(e, now)
	if len(e.CoupleUserIDs)+len(invites) >= 2 {
		return nil, ErrCoupleCapacity
	}
	for _, i := range invites {
		if i.Email == invite.Email {
			return nil, ErrCoupleCapacity
		}
	}
	invites[invite.TokenHash] = invite
	return invites, nil
}
func AcceptCoupleInvite(e Event, tokenHash string, u User, now time.Time) ([]string, map[string]CoupleInvite, error) {
	invite, ok := e.CoupleInvites[tokenHash]
	if !ok || invite.Accepted || !invite.ExpiresAt.After(now) || invite.Email != u.Email || u.GoogleSub == "" || u.IdentityPolicy != 1 || !u.GoogleAuthoritative {
		return nil, nil, ErrCoupleAccess
	}
	if e.Kind != "wedding" || e.PaymentStatus != "paid" || e.IsDemo {
		return nil, nil, ErrUnavailable
	}
	invites := ActiveCoupleInvites(e, now)
	delete(invites, tokenHash)
	for _, id := range e.CoupleUserIDs {
		if id == u.ID {
			return e.CoupleUserIDs, invites, nil
		}
	}
	if len(e.CoupleUserIDs) >= 2 {
		return nil, nil, ErrCoupleCapacity
	}
	return append(append([]string{}, e.CoupleUserIDs...), u.ID), invites, nil
}
func AttachCouple(e Event, id string, now time.Time) ([]string, error) {
	if e.Kind != "wedding" || e.PaymentStatus != "paid" || e.IsDemo {
		return nil, ErrUnavailable
	}
	if len(e.CoupleUserIDs)+len(ActiveCoupleInvites(e, now)) >= 2 {
		return nil, ErrCoupleCapacity
	}
	return append(append([]string{}, e.CoupleUserIDs...), id), nil
}
