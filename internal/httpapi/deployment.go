package httpapi

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/dokploy"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"log"
	"net/http"
	"os"
	"time"
)

func deploymentClient() (*dokploy.Client, error) {
	return dokploy.New(os.Getenv("DOKPLOY_URL"), os.Getenv("DOKPLOY_THE_DATE_API_KEY"), env("INVITATION_API_URL", "https://api.thedate.now"))
}
func (s *server) publish(w http.ResponseWriter, r *http.Request) {
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	if e.IsDemo || e.PaymentStatus != "paid" {
		bad(w, 403, "Complete checkout before publishing")
		return
	}
	image := (mongostore.Deployments{DB: s.db}).RendererImage(r.Context(), os.Getenv("DOKPLOY_INVITATION_IMAGE"))
	if _, err = deploymentClient(); err != nil || !dokploy.ValidImage(image) {
		bad(w, 503, "Invitation deployment is not configured")
		return
	}
	d, err := (mongostore.Deployments{DB: s.db}).Queue(r.Context(), e, image)
	if err != nil {
		bad(w, 500, "Could not queue deployment")
		return
	}
	reply(w, http.StatusAccepted, s.publicationView(r.Context(), e, d))
}
func (s *server) deploymentStatus(w http.ResponseWriter, r *http.Request) {
	e, err := s.managedEvent(r)
	if err != nil {
		bad(w, 404, "Event not found")
		return
	}
	d, err := (mongostore.Deployments{DB: s.db}).Get(r.Context(), e.ID)
	if err != nil {
		bad(w, 404, "Deployment not found")
		return
	}
	reply(w, 200, s.publicationView(r.Context(), e, d))
}
func (s *server) deploymentWorker(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	releaseTicker := time.NewTicker(time.Minute)
	defer releaseTicker.Stop()
	s.refreshRenderer(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-releaseTicker.C:
			s.refreshRenderer(ctx)
		case <-ticker.C:
			s.deploymentStep(ctx)
		}
	}
}
func (s *server) deploymentStep(parent context.Context) {
	provider, err := deploymentClient()
	if err != nil {
		return
	}
	repo := mongostore.Deployments{DB: s.db}
	ctx, cancel := context.WithTimeout(parent, 100*time.Second)
	defer cancel()
	d, claim, err := repo.Claim(ctx)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return
	}
	if err != nil {
		log.Print("deployment claim unavailable")
		return
	}
	var e core.Event
	err = s.db.Collection("events").FindOne(ctx, bson.M{"_id": d.EventID, "paymentStatus": "paid", "isDemo": bson.M{"$ne": true}}).Decode(&e)
	if err == nil && d.Phase != "ready" {
		d, err = application.Advance(ctx, provider, e, d, func(next core.Deployment) error { return repo.Save(ctx, next, claim) })
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	if err != nil {
		d.Phase = "error"
		d.Error = "Deployment failed; retry publication"
		if saveErr := repo.Save(finishCtx, d, claim); saveErr != nil {
			log.Print("deployment checkpoint unavailable")
			return
		}
	}
	if err = repo.Finish(finishCtx, d, claim); err != nil {
		log.Print("deployment completion unavailable")
	}
}
