package application

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"maps"
	"time"
)

type ManagedResponseRepository interface {
	Snapshot(context.Context, string) (core.Event, error)
	CommitManagedResponse(context.Context, core.Event, string, core.Response) (bool, error)
}
type ManagedResponses struct {
	Repository ManagedResponseRepository
	Now        func() time.Time
}

func (s ManagedResponses) Correct(ctx context.Context, eventID string, g core.Guest, choice, reason string) error {
	for attempt := 0; attempt < 32; attempt++ {
		e, err := s.Repository.Snapshot(ctx, eventID)
		if err != nil {
			return err
		}
		if e.RevokedGuestTokens[g.InviteToken] {
			return core.ErrUnavailable
		}
		// Organizers can correct a closed response, while keeping party and capacity rules.
		editable := e
		editable.Responses = maps.Clone(e.Responses)
		previous := editable.Responses[g.ID]
		previous.Finalized = false
		if editable.Responses == nil {
			editable.Responses = map[string]core.Response{}
		}
		editable.Responses[g.ID] = previous
		now := s.Now()
		editable.PublishedAt = &now
		editable.InvitationsEnabled = false
		next, err := core.DecidePartyResponse(editable, g, choice, reason, nil, now)
		if err != nil {
			return err
		}
		next.Finalized = choice == "not_going" || (choice == "going" && next.PartyRegistered)
		next.ProviderTimestamp = previous.ProviderTimestamp
		saved, err := s.Repository.CommitManagedResponse(ctx, e, g.ID, next)
		if err != nil {
			return err
		}
		if saved {
			return nil
		}
	}
	return ErrConflict
}
