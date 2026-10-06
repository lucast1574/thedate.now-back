package dokploy

import (
	"context"
	"encoding/json"
	appservice "github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestNexodeFlowCheckpointsAndReusesInvitationResources(t *testing.T) {
	var mu sync.Mutex
	projectCreated, appCreated, domainCreated := false, false, false
	calls := map[string]int{}
	envContent := ""
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		endpoint := strings.TrimPrefix(r.URL.Path, "/api/")
		calls[endpoint]++
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing centralized authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		switch endpoint {
		case "project.all":
			list := []project{}
			if projectCreated {
				env := environment{ID: "environment"}
				if appCreated {
					env.Applications = []application{{ID: "app", Name: "invitation-event"}}
				}
				list = append(list, project{ID: "project", Name: "date-invitation-event", Environments: []environment{env}})
			}
			_ = json.NewEncoder(w).Encode(list)
		case "project.create":
			projectCreated = true
			_, _ = w.Write([]byte(`{"project":{"projectId":"project"},"environment":{"environmentId":"environment"}}`))
		case "application.create":
			appCreated = true
			_, _ = w.Write([]byte(`{"applicationId":"app"}`))
		case "application.saveEnvironment":
			var in struct {
				Env string `json:"env"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			envContent = in.Env
			_, _ = w.Write([]byte(`{}`))
		case "domain.byApplicationId":
			if domainCreated {
				_, _ = w.Write([]byte(`[{"domainId":"domain","host":"ana.save.thedate.now"}]`))
			} else {
				_, _ = w.Write([]byte(`[]`))
			}
		case "domain.create":
			domainCreated = true
			_, _ = w.Write([]byte(`{"domainId":"domain"}`))
		case "application.one":
			_, _ = w.Write([]byte(`{"applicationId":"app","applicationStatus":"idle"}`))
		case "application.saveDockerProvider", "application.update", "application.deploy":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected operation %s", endpoint)
			w.WriteHeader(404)
		}
	}))
	defer provider.Close()
	client, err := New(provider.URL, "test-key", "https://api.thedate.now")
	if err != nil {
		t.Fatal(err)
	}
	client.HTTP = provider.Client()
	image := "ghcr.io/example/invitation@sha256:" + strings.Repeat("a", 64)
	e := core.Event{ID: "event", Kind: "wedding", Slug: "ana"}
	d := core.Deployment{EventID: e.ID, Host: "ana.save.thedate.now", Image: image, Phase: "pending"}
	var checkpoint core.Deployment
	d, err = appservice.Advance(context.Background(), client, e, d, func(next core.Deployment) error { checkpoint = next; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if d.Phase != "deploying" || checkpoint.ApplicationID != "app" || checkpoint.DomainID != "domain" {
		t.Fatal("missing durable checkpoint")
	}
	// A new process resumes polling, without recreating resources or redeploying.
	_, err = appservice.Advance(context.Background(), client, e, checkpoint, func(next core.Deployment) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	// Reconciliation also reuses resources if a response/checkpoint was lost.
	if id, _, err := client.EnsureProject(context.Background(), "date-invitation-event"); err != nil || id != "project" {
		t.Fatal(err)
	}
	if id, err := client.EnsureApplication(context.Background(), "invitation-event", "environment"); err != nil || id != "app" {
		t.Fatal(err)
	}
	if id, err := client.EnsureDomain(context.Background(), "app", "ana.save.thedate.now"); err != nil || id != "domain" {
		t.Fatal(err)
	}
	for _, ep := range []string{"project.create", "application.create", "domain.create", "application.deploy"} {
		if calls[ep] != 1 {
			t.Fatalf("duplicate %s: %d", ep, calls[ep])
		}
	}
	if !strings.Contains(envContent, "INVITATION_KIND=wedding") || !strings.Contains(envContent, "INVITATION_EVENT_ID=event") || strings.Contains(envContent, "JWT_SECRET") || strings.Contains(envContent, "test-key") {
		t.Fatal("invalid isolated runtime config")
	}
}
func TestProviderErrorsNeverExposeResponseSecretsAndNeverRetryCreates(t *testing.T) {
	calls := 0
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(500)
		_, _ = w.Write([]byte("secret-key-from-provider"))
	}))
	defer provider.Close()
	c, err := New(provider.URL, "test-key", "https://api.thedate.now")
	if err != nil {
		t.Fatal(err)
	}
	c.HTTP = provider.Client()
	err = c.call(context.Background(), "project.create", map[string]string{"name": "test"}, nil)
	if err == nil || strings.Contains(err.Error(), "secret-key") || calls != 1 {
		t.Fatal("unsafe mutation retries or secret leakage")
	}
	for _, image := range []string{"image:latest", "image;echo token", "image@sha256:abc"} {
		if ValidImage(image) {
			t.Fatal("unsafe or mutable image accepted")
		}
	}
	if _, err = New("http://dokploy.test", "key", "https://api.thedate.now"); err == nil {
		t.Fatal("plaintext control-plane URL accepted")
	}
}
