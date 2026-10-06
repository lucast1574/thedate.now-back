package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/dokploy"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"log"
	"os"
	"time"
)

func (s *server) refreshRenderer(parent context.Context) {
	manifest := os.Getenv("DOKPLOY_RENDERER_MANIFEST_URL")
	if manifest == "" {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	provider, err := deploymentClient()
	if err != nil {
		return
	}
	image, err := dokploy.ResolveRenderer(ctx, provider.HTTP, os.Getenv("DOKPLOY_RENDERER_IMAGE_REPOSITORY"), manifest)
	if err != nil {
		log.Print("renderer release unavailable; keeping current image")
		return
	}
	if err = (mongostore.Deployments{DB: s.db}).RollRenderer(ctx, image); err != nil {
		log.Print("renderer release checkpoint unavailable")
	}
}
