package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"time"
)

var errCapacity = core.ErrCapacity

func (s *server) responses() application.Responses {
	return application.Responses{Repository: mongostore.Responses{DB: s.db}, Now: func() time.Time { return time.Now().UTC() }}
}
func (s *server) applyResponse(ctx context.Context, e core.Event, g core.Guest, response, reason string) error {
	return s.responses().Apply(ctx, e.ID, g, response, reason, "")
}
