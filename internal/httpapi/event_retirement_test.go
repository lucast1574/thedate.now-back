package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRetirementStopsOnlyExpiredSpacesAndPreservesInvitation(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	projects := []any{}
	if err := mongostore.EnsurePaidAddressIndex(ctx, db); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"general", "wedding"} {
		for _, state := range []string{"past", "future", "demo"} {
			id := kind + state
			e := core.Event{ID: id, Kind: kind, Slug: id, PaymentStatus: "paid", Title: "Saved invitation", IsDemo: state == "demo", StartAt: time.Now().AddDate(0, 0, -2), TimeZone: "America/Bogota", InvitationsEnabled: true}
			if state == "future" {
				e.StartAt = time.Now().AddDate(0, 0, 2)
			}
			if _, err := db.Collection("events").InsertOne(ctx, e); err != nil {
				t.Fatal(err)
			}
			host, _ := core.InvitationHost(kind, id)
			d := core.Deployment{EventID: id, ProjectID: "p" + id, EnvironmentID: "env" + id, ApplicationID: "app" + id, DomainID: "domain" + id, Host: host, Phase: "ready"}
			if kind == "general" && state == "past" {
				d.Phase = "stopped"
			}
			if _, err := db.Collection("deployments").InsertOne(ctx, d); err != nil {
				t.Fatal(err)
			}
			projects = append(projects, map[string]any{"projectId": d.ProjectID, "name": "date-invitation-" + id, "environments": []any{map[string]any{"environmentId": d.EnvironmentID, "applications": []any{map[string]string{"applicationId": d.ApplicationID, "name": "invitation-" + id}}}}})
		}
	}
	stopped := map[string]bool{}
	removed := map[string]bool{}
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/project.all" {
			_ = json.NewEncoder(w).Encode(projects)
			return
		}
		if r.URL.Path == "/api/domain.byApplicationId" {
			id := r.URL.Query().Get("applicationId")[3:]
			kind := "general"
			if id == "weddingpast" {
				kind = "wedding"
			}
			host, _ := core.InvitationHost(kind, id)
			if removed["domain"+id] {
				w.Write([]byte(`[]`))
			} else {
				_ = json.NewEncoder(w).Encode([]any{map[string]string{"domainId": "domain" + id, "host": host}})
			}
			return
		}
		if r.URL.Path == "/api/domain.delete" {
			var in struct {
				ID string `json:"domainId"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			removed[in.ID] = true
			w.Write([]byte(`{}`))
			return
		}
		if r.URL.Path != "/api/application.stop" {
			t.Errorf("unexpected call %s", r.URL.Path)
			w.WriteHeader(400)
			return
		}
		var in struct {
			ApplicationID string `json:"applicationId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		stopped[in.ApplicationID] = true
		w.Write([]byte(`{}`))
	}))
	defer provider.Close()
	previous := http.DefaultTransport
	http.DefaultTransport = provider.Client().Transport
	defer func() { http.DefaultTransport = previous }()
	t.Setenv("DOKPLOY_URL", provider.URL)
	t.Setenv("DOKPLOY_THE_DATE_API_KEY", "test")
	s := &server{db: db}
	s.retireExpiredEvents(ctx)
	s.retireExpiredEvents(ctx)
	if len(stopped) != 2 || !stopped["appgeneralpast"] || !stopped["appweddingpast"] {
		t.Fatalf("incorrect stopped spaces: %v", stopped)
	}
	if len(removed) != 2 {
		t.Fatalf("routes not removed: %v", removed)
	}
	for _, kind := range []string{"general", "wedding"} {
		var e core.Event
		var d core.Deployment
		if err := db.Collection("events").FindOne(ctx, bson.M{"_id": kind + "past"}).Decode(&e); err != nil {
			t.Fatal(err)
		}
		if err := db.Collection("deployments").FindOne(ctx, bson.M{"_id": e.ID}).Decode(&d); err != nil {
			t.Fatal(err)
		}
		if e.Title != "Saved invitation" || e.InvitationsEnabled || d.Phase != "stopped" || e.AddressReleasedAt == nil || e.Slug != kind+"past" {
			t.Fatal("invitation lost or space not stopped")
		}
		now := time.Now().UTC()
		next := core.Event{ID: "next-" + kind, Kind: kind, Slug: e.Slug, Title: "New celebration", PaymentStatus: "paid", PublishedAt: &now, StartAt: now.AddDate(0, 0, 2)}
		if _, err := db.Collection("events").InsertOne(ctx, next); err != nil {
			t.Fatalf("address cannot be reused: %v", err)
		}
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest("GET", "/public/events/"+kind+"/"+e.Slug, nil))
		var view map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &view)
		if w.Code != 200 || view["id"] != next.ID {
			t.Fatalf("reused address resolved old event: %s", w.Body.String())
		}
	}
}
