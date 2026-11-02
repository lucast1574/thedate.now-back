package dokploy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lucast1574/thedate.now-back/internal/core"
)

func TestStopInvitationVerifiesItsDedicatedProject(t *testing.T) {
	stops := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/project.all":
			w.Write([]byte(`[{"projectId":"project","name":"date-invitation-event","environments":[{"environmentId":"env","applications":[{"applicationId":"app","name":"invitation-event"}]}]}]`))
		case "/api/application.stop":
			stops++
			w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c, err := New(server.URL, "test", "https://api.example.com")
	if err != nil {
		t.Fatal(err)
	}
	c.HTTP = server.Client()
	d := core.Deployment{EventID: "event", ProjectID: "project", EnvironmentID: "env", ApplicationID: "app"}
	if err := c.StopInvitation(context.Background(), d); err != nil || stops != 1 {
		t.Fatalf("stop: %v (%d)", err, stops)
	}
	d.EventID = "other"
	if err := c.StopInvitation(context.Background(), d); err == nil || stops != 1 {
		t.Fatal("stopped unrelated service")
	}
}
