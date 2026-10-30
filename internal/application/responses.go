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
	return s.applyParty(ctx, eventID, g, choice, reason, receipt, companions, false, 0)
}
func (s Responses) FinalizeParty(ctx context.Context, eventID string, g core.Guest, choice, reason string, companions *[]core.Person) error {
	return s.applyParty(ctx, eventID, g, choice, reason, "", companions, true, 0)
}
func (s Responses) ApplyProvider(ctx context.Context, eventID string, g core.Guest, choice, receipt string, timestamp int64) error {
	return s.applyParty(ctx, eventID, g, choice, "", receipt, nil, false, timestamp)
}
func (s Responses) applyParty(ctx context.Context, eventID string, g core.Guest, choice, reason, receipt string, companions *[]core.Person, finalize bool, timestamp int64) error {
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
		if (finalize || receipt != "") && core.ResponseCompleted(e.Responses[g.ID]) {
			return core.ErrResponseFinalized
		}
		if timestamp > 0 && e.Responses[g.ID].ProviderTimestamp >= timestamp {
			return core.ErrDuplicate
		}
		nextReason := reason
		if choice == "maybe" && nextReason == "" {
			nextReason = e.Responses[g.ID].Reason
		}
		next, err := core.DecidePartyResponse(e, g, choice, nextReason, companions, s.Now())
		if err != nil {
			return err
		}
		next.Finalized = (finalize && choice != "maybe") || choice == "not_going"
		next.ProviderTimestamp = e.Responses[g.ID].ProviderTimestamp
		if timestamp > 0 {
			next.ProviderTimestamp = timestamp
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
		if in.Slug != e.Slug && (e.PublishedAt != nil || e.InvitationsEnabled || core.ValidateSlug(in.Slug) != nil) {
			return core.ErrPublishedAddress
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
