package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"time"
)

type publicationView struct {
	EventID string `json:"eventId"`
	Phase   string `json:"phase"`
	Host    string `json:"host,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (s *server) publicationView(ctx context.Context, e core.Event, d core.Deployment) publicationView {
	view := publicationView{EventID: d.EventID, Phase: d.Phase}
	if d.Phase == "error" {
		view.Error = "Invitation could not be published; your changes are saved"
	}
	if d.Phase != "ready" {
		return view
	}
	view.Phase = "verifying"
	client, err := deploymentClient()
	if err != nil {
		return view
	}
	checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if client.PublicPageReady(checkCtx, e, d) {
		view.Phase = "ready"
		view.Host = d.Host
	}
	return view
}
