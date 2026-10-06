package core

import (
	"errors"
	"strings"
	"time"
)

var ErrCapacity = errors.New("event capacity reached")
var ErrDuplicate = errors.New("response receipt already processed")
var ErrUnavailable = errors.New("event is not published and paid")

type Response struct {
	Companions      []Person   `bson:"companions" json:"-"`
	PartyRegistered bool       `bson:"partyRegistered" json:"-"`
	Seats           int        `bson:"seats" json:"-"`
	Choice          string     `bson:"choice" json:"-"`
	Reason          string     `bson:"reason" json:"-"`
	ExpiresAt       *time.Time `bson:"expiresAt" json:"-"`
	RespondedAt     time.Time  `bson:"respondedAt" json:"-"`
}

func (r Response) Active(now time.Time) bool {
	return r.Choice == "going" || (r.Choice == "maybe" && r.ExpiresAt != nil && r.ExpiresAt.After(now))
}
func Occupied(e Event, now time.Time) int {
	total := 0
	for _, r := range e.Responses {
		if r.Active(now) {
			total += r.Seats
		}
	}
	return total
}

// DecideResponse is pure: IO, retries and persistence belong to the application.
func DecideResponse(e Event, g Guest, choice, reason string, now time.Time) (Response, error) {
	if e.PaymentStatus != "paid" || e.PublishedAt == nil {
		return Response{}, ErrUnavailable
	}
	if choice != "going" && choice != "maybe" && choice != "not_going" {
		return Response{}, errors.New("invalid response")
	}
	if g.EventID != e.ID || g.Seats < 1 || g.Seats > 20 || len(reason) > 2000 {
		return Response{}, errors.New("invalid guest response")
	}
	next := Response{Seats: g.Seats, Choice: choice, Reason: strings.TrimSpace(reason), RespondedAt: now}
	if choice == "maybe" {
		expiry := now.Add(time.Duration(e.MaybeHoldHours) * time.Hour)
		next.ExpiresAt = &expiry
	}
	used := Occupied(e, now)
	if old := e.Responses[g.ID]; old.Active(now) {
		used -= old.Seats
	}
	if next.Active(now) && !e.CapacityUnlimited && used+g.Seats > e.Capacity {
		return Response{}, ErrCapacity
	}
	return next, nil
}
func WithResponse(e Event, g Guest) Guest {
	if response, ok := e.Responses[g.ID]; ok {
		g.Response = response.Choice
		g.AttendingSeats = response.Seats
		g.PartyRegistered = response.PartyRegistered
		if response.PartyRegistered || response.Companions != nil {
			g.Companions = response.Companions
		}
		g.MaybeReason = response.Reason
		g.MaybeExpiresAt = response.ExpiresAt
		g.RespondedAt = &response.RespondedAt
	}
	return g
}
