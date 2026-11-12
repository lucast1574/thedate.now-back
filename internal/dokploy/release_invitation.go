package dokploy

import (
	"context"
	"net/url"

	"github.com/lucast1574/thedate.now-back/internal/core"
)

// Verify the dedicated application's ownership before removing only its route.
func (c *Client) RetireInvitation(ctx context.Context, d core.Deployment) error {
	if err := c.StopInvitation(ctx, d); err != nil {
		return err
	}
	if d.ApplicationID == "" {
		return nil
	}
	var domains []struct {
		ID   string `json:"domainId"`
		Host string `json:"host"`
	}
	if err := c.call(ctx, "domain.byApplicationId", url.Values{"applicationId": {d.ApplicationID}}, &domains); err != nil {
		return err
	}
	for _, domain := range domains {
		if domain.Host == d.Host {
			if err := c.call(ctx, "domain.delete", map[string]string{"domainId": domain.ID}, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
