package application

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"time"
)

var ErrConflict = errors.New("concurrent changes; retry")

type ResponseRepository interface {
	Snapshot(context.Context, string) (core.Event, error)
	CommitResponse(context.Context, core.Event, string, core.Response, string) (bool, error)
	CommitEvent(context.Context, core.Event, core.EventInput, time.Time) (bool, error)
}
type Responses struct {
	Repository ResponseRepository
	Now        func() time.Time
}

func (s Responses) Apply(ctx context.Context, eventID string, g core.Guest, choice, reason, receipt string) error {
	return s.ApplyParty(ctx, eventID, g, choice, reason, receipt, nil)
}
func (s Responses) ApplyParty(ctx context.Context, eventID string, g core.Guest, choice, reason, receipt string, companions *[]core.Person) error {
	for attempt := 0; attempt < 32; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		e, err := s.Repository.Snapshot(ctx, eventID)
		if err != nil {
			return err
		}
		if receipt != "" && e.ResponseReceipts[receipt] {
			return core.ErrDuplicate
		}
		next, err := core.DecidePartyResponse(e, g, choice, reason, companions, s.Now())
		if err != nil {
			return err
		}
		saved, err := s.Repository.CommitResponse(ctx, e, g.ID, next, receipt)
		if err != nil {
			return err
		}
		if saved {
			return nil
		}
	}
	return ErrConflict
}
func (s Responses) UpdateEvent(ctx context.Context, id string, in core.EventInput) error {
	if err := core.ValidateEvent(in, false); err != nil {
		return err
	}
	in = core.NormalizeEvent(in)
	for attempt := 0; attempt < 32; attempt++ {
		e, err := s.Repository.Snapshot(ctx, id)
		if err != nil {
			return err
		}
		if !in.CapacityUnlimited && (core.Occupied(e, s.Now()) > in.Capacity || (e.Seating != nil && len(e.Seating.Assignments) > in.Capacity)) {
			return core.ErrCapacity
		}
		saved, err := s.Repository.CommitEvent(ctx, e, in, s.Now())
		if err != nil {
			return err
		}
		if saved {
			return nil
		}
	}
	return ErrConflict
}
