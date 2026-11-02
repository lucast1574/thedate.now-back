package dokploy

import (
	"context"
	"fmt"

	"github.com/lucast1574/thedate.now-back/internal/core"
)

// Only stop the application belonging to this event's dedicated project.
func (c *Client) StopInvitation(ctx context.Context, d core.Deployment) error {
	if d.ApplicationID == "" {
		return nil
	}
	projects, err := c.projects(ctx)
	if err != nil {
		return err
	}
	for _, p := range projects {
		if p.ID != d.ProjectID || p.Name != "date-invitation-"+d.EventID {
			continue
		}
		for _, env := range p.Environments {
			if env.ID != d.EnvironmentID {
				continue
			}
			for _, app := range env.Applications {
				if app.ID == d.ApplicationID && app.Name == "invitation-"+d.EventID {
					return c.call(ctx, "application.stop", map[string]string{"applicationId": app.ID}, nil)
				}
			}
		}
	}
	return fmt.Errorf("invitation application ownership could not be verified")
}
