package application

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
)

type DeploymentProvider interface {
	EnsureProject(context.Context, string) (string, string, error)
	EnsureApplication(context.Context, string, string) (string, error)
	Configure(context.Context, string, core.Event, string) error
	EnsureDomain(context.Context, string, string) (string, error)
	Deploy(context.Context, string) error
	Ready(context.Context, core.Deployment) (bool, error)
}

// Advance checkpoints each external resource before continuing. Create calls are
// reconciled by deterministic names, never blindly retried after an unknown result.
func Advance(ctx context.Context, p DeploymentProvider, e core.Event, d core.Deployment, save func(core.Deployment) error) (core.Deployment, error) {
	var err error
	if d.ProjectID == "" {
		d.ProjectID, d.EnvironmentID, err = p.EnsureProject(ctx, "date-invitation-"+e.ID)
		if err != nil {
			return d, err
		}
		if err = save(d); err != nil {
			return d, err
		}
	}
	if d.ApplicationID == "" {
		d.ApplicationID, err = p.EnsureApplication(ctx, "invitation-"+e.ID, d.EnvironmentID)
		if err != nil {
			return d, err
		}
		if err = save(d); err != nil {
			return d, err
		}
	}
	if d.Phase != "deploying" {
		if err = p.Configure(ctx, d.ApplicationID, e, d.Image); err != nil {
			return d, err
		}
		d.DomainID, err = p.EnsureDomain(ctx, d.ApplicationID, d.Host)
		if err != nil {
			return d, err
		}
		d.Phase = "configured"
		if err = save(d); err != nil {
			return d, err
		}
		if err = p.Deploy(ctx, d.ApplicationID); err != nil {
			return d, err
		}
		d.Phase = "deploying"
		if err = save(d); err != nil {
			return d, err
		}
	}
	ready, err := p.Ready(ctx, d)
	if err != nil {
		return d, err
	}
	if ready {
		d.Phase = "ready"
		err = save(d)
	}
	return d, err
}
