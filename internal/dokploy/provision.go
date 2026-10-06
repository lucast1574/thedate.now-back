package dokploy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"net/http"
	"net/url"
)

type environment struct {
	ID           string        `json:"environmentId"`
	Applications []application `json:"applications"`
}
type project struct {
	ID           string        `json:"projectId"`
	Name         string        `json:"name"`
	Environments []environment `json:"environments"`
}
type application struct {
	ID     string `json:"applicationId"`
	Name   string `json:"name"`
	Status string `json:"applicationStatus"`
}

func (c *Client) projects(ctx context.Context) ([]project, error) {
	var list []project
	err := c.call(ctx, "project.all", url.Values{}, &list)
	return list, err
}
func (c *Client) EnsureProject(ctx context.Context, name string) (string, string, error) {
	list, err := c.projects(ctx)
	if err != nil {
		return "", "", err
	}
	for _, p := range list {
		if p.Name == name && len(p.Environments) > 0 {
			return p.ID, p.Environments[0].ID, nil
		}
	}
	var created struct {
		Project     project     `json:"project"`
		Environment environment `json:"environment"`
	}
	err = c.call(ctx, "project.create", map[string]string{"name": name, "description": "The Date managed invitation"}, &created)
	if err != nil {
		return "", "", err
	}
	if created.Project.ID != "" && created.Environment.ID != "" {
		return created.Project.ID, created.Environment.ID, nil
	}
	list, err = c.projects(ctx)
	if err != nil {
		return "", "", err
	}
	for _, p := range list {
		if p.Name == name && len(p.Environments) > 0 {
			return p.ID, p.Environments[0].ID, nil
		}
	}
	return "", "", fmt.Errorf("project has no environment")
}
func (c *Client) EnsureApplication(ctx context.Context, name, env string) (string, error) {
	list, err := c.projects(ctx)
	if err != nil {
		return "", err
	}
	for _, p := range list {
		for _, e := range p.Environments {
			if e.ID == env {
				for _, a := range e.Applications {
					if a.Name == name {
						return a.ID, nil
					}
				}
			}
		}
	}
	var a application
	err = c.call(ctx, "application.create", map[string]string{"name": name, "environmentId": env}, &a)
	if err == nil && a.ID == "" {
		err = fmt.Errorf("application ID missing")
	}
	return a.ID, err
}
func (c *Client) Configure(ctx context.Context, id string, e core.Event, image string) error {
	if !ValidImage(image) {
		return fmt.Errorf("invitation image must use an immutable sha256 digest")
	}
	if err := c.call(ctx, "application.saveDockerProvider", map[string]any{"applicationId": id, "dockerImage": image, "username": nil, "password": nil, "registryUrl": nil}, nil); err != nil {
		return err
	}
	env := fmt.Sprintf("PORT=3000\nHOSTNAME=0.0.0.0\nAPI_INTERNAL_URL=%s\nNEXT_PUBLIC_API_URL=%s\nINVITATION_KIND=%s\nINVITATION_SLUG=%s\nINVITATION_EVENT_ID=%s\nINVITATION_RENDERER_IMAGE=%s\n", c.APIURL, c.APIURL, e.Kind, e.Slug, e.ID, image)
	if err := c.call(ctx, "application.saveEnvironment", map[string]any{"applicationId": id, "env": env, "buildArgs": "", "buildSecrets": "", "createEnvFile": false}, nil); err != nil {
		return err
	}
	return c.call(ctx, "application.update", map[string]any{"applicationId": id, "replicas": 1, "autoDeploy": false, "cpuLimit": "500000000", "memoryLimit": "536870912"}, nil)
}
func (c *Client) EnsureDomain(ctx context.Context, id, host string) (string, error) {
	var domains []struct {
		ID   string `json:"domainId"`
		Host string `json:"host"`
	}
	if err := c.call(ctx, "domain.byApplicationId", url.Values{"applicationId": {id}}, &domains); err != nil {
		return "", err
	}
	for _, d := range domains {
		if d.Host == host {
			return d.ID, nil
		}
	}
	var created struct {
		ID string `json:"domainId"`
	}
	err := c.call(ctx, "domain.create", map[string]any{"host": host, "https": true, "certificateType": "letsencrypt", "port": 3000, "applicationId": id}, &created)
	if err == nil && created.ID == "" {
		err = fmt.Errorf("domain ID missing")
	}
	return created.ID, err
}
func (c *Client) Deploy(ctx context.Context, id string) error {
	return c.call(ctx, "application.deploy", map[string]string{"applicationId": id}, nil)
}
func (c *Client) Ready(ctx context.Context, d core.Deployment) (bool, error) {
	var app application
	if err := c.call(ctx, "application.one", url.Values{"applicationId": {d.ApplicationID}}, &app); err != nil {
		return false, err
	}
	if app.Status == "error" {
		return false, fmt.Errorf("invitation deployment failed")
	}
	if app.Status != "done" && app.Status != "running" {
		return false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+d.Host+"/api/invitation-health", nil)
	if err != nil {
		return false, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	var health struct {
		EventID string `json:"eventId"`
		Image   string `json:"image"`
	}
	return resp.StatusCode == 200 && json.NewDecoder(resp.Body).Decode(&health) == nil && health.EventID == d.EventID && health.Image == d.Image, nil
}
