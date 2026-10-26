package application

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"time"
)

type WishlistRepository interface {
	Snapshot(context.Context, string) (core.Event, error)
	Commit(context.Context, string, int64, core.Wishlist, bool) (bool, error)
}
type Wishlists struct {
	Repository WishlistRepository
	Now        func() time.Time
}

func (s Wishlists) Choose(ctx context.Context, g core.Guest, id string, release bool) error {
	for attempt := 0; attempt < 32; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		e, err := s.Repository.Snapshot(ctx, g.EventID)
		if err != nil {
			return err
		}
		next, err := core.DecideGift(e, g, id, release, s.Now())
		if err != nil {
			return err
		}
		saved, err := s.Repository.Commit(ctx, e.ID, e.Wishlist.Version, next, true)
		if err != nil {
			return err
		}
		if saved {
			return nil
		}
	}
	return ErrConflict
}
