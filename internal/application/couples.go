package application

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"time"
)

type CoupleRepository interface {
	Snapshot(context.Context, string) (core.Event, error)
	Save(context.Context, core.Event, []string, map[string]core.CoupleInvite) (bool, error)
}
type Couples struct {
	Repository CoupleRepository
	Now        func() time.Time
}

func (s Couples) change(ctx context.Context, id string, decide func(core.Event) ([]string, map[string]core.CoupleInvite, error)) error {
	for i := 0; i < 32; i++ {
		e, err := s.Repository.Snapshot(ctx, id)
		if err != nil {
			return err
		}
		ids, invites, err := decide(e)
		if err != nil {
			return err
		}
		saved, err := s.Repository.Save(ctx, e, ids, invites)
		if err != nil {
			return err
		}
		if saved {
			return nil
		}
	}
	return ErrConflict
}
func (s Couples) Invite(ctx context.Context, id string, invite core.CoupleInvite) error {
	return s.change(ctx, id, func(e core.Event) ([]string, map[string]core.CoupleInvite, error) {
		invites, err := core.AddCoupleInvite(e, invite, s.Now())
		return e.CoupleUserIDs, invites, err
	})
}
func (s Couples) Accept(ctx context.Context, id, hash string, u core.User) error {
	return s.change(ctx, id, func(e core.Event) ([]string, map[string]core.CoupleInvite, error) {
		return core.AcceptCoupleInvite(e, hash, u, s.Now())
	})
}
func (s Couples) Attach(ctx context.Context, id, userID string) error {
	return s.change(ctx, id, func(e core.Event) ([]string, map[string]core.CoupleInvite, error) {
		ids, err := core.AttachCouple(e, userID, s.Now())
		return ids, core.ActiveCoupleInvites(e, s.Now()), err
	})
}
