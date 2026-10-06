package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/url"
	"strings"
)

func (s *server) completeLegacyEvent(ctx context.Context, event *core.Event) {
	if strings.TrimSpace(event.Organizer) == "" {
		var owner struct {
			Name string `bson:"name"`
		}
		if s.db.Collection("users").FindOne(ctx, bson.M{"_id": event.OwnerID}).Decode(&owner) == nil {
			event.Organizer = owner.Name
		}
		if strings.TrimSpace(event.Organizer) == "" {
			event.Organizer = event.Title
		}
	}
	if !event.IsVirtual && event.MapURL == "" && strings.TrimSpace(event.Location) != "" {
		event.MapURL = "https://www.google.com/maps/search/?api=1&query=" + url.QueryEscape(event.Location)
	}
}
