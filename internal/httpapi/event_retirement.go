package httpapi

import (
	"context"
	"log"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *server) retireExpiredEvents(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 50*time.Second)
	defer cancel()
	provider, err := deploymentClientFor(s.product)
	if err != nil {
		return
	}
	cur, err := s.db.Collection("events").Find(ctx, bson.M{"isDemo": bson.M{"$ne": true}, "startAt": bson.M{"$lte": time.Now().UTC()}})
	if err != nil {
		return
	}
	defer cur.Close(ctx)
	repo := mongostore.Deployments{DB: s.db}
	for cur.Next(ctx) {
		var e core.Event
		if cur.Decode(&e) != nil || !core.EventClosed(e, time.Now().UTC()) {
			continue
		}
		_, err = s.db.Collection("events").UpdateOne(ctx, bson.M{"_id": e.ID, "startAt": e.StartAt, "timeZone": e.TimeZone}, bson.M{"$set": bson.M{"invitationsEnabled": false, "invitationWebReady": false}})
		if err != nil {
			continue
		}
		d, claim, err := repo.ClaimClosure(ctx, e.ID, time.Now().UTC())
		if err != nil {
			continue
		}
		if err = provider.StopInvitation(ctx, d); err != nil {
			log.Print("invitation retirement pending")
			continue
		}
		d.Phase, d.Error = "stopped", ""
		if repo.Save(ctx, d, claim) != nil || repo.Finish(ctx, d, claim) != nil {
			log.Print("invitation retirement checkpoint pending")
		}
	}
}
