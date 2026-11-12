package httpapi

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (s *server) retireExpiredEvents(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 50*time.Second)
	defer cancel()
	provider, err := deploymentClientFor(s.product)
	if err != nil {
		return
	}
	cur, err := s.db.Collection("events").Find(ctx, bson.M{"addressReleasedAt": nil, "isDemo": bson.M{"$ne": true}, "startAt": bson.M{"$lte": time.Now().UTC()}})
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
		if _, err := repo.Get(ctx, e.ID); errors.Is(err, mongo.ErrNoDocuments) {
			if err := repo.ReleaseAddress(ctx, e, time.Now().UTC()); err != nil {
				log.Print("unused invitation address release pending")
			}
			continue
		}
		d, claim, err := repo.ClaimClosure(ctx, e.ID, time.Now().UTC())
		if err != nil {
			continue
		}
		if err = provider.RetireInvitation(ctx, d); err != nil {
			log.Print("invitation retirement pending")
			continue
		}
		d.Phase, d.Error = "stopped", ""
		d.DomainID = ""
		if repo.Save(ctx, d, claim) != nil || repo.Finish(ctx, d, claim) != nil {
			log.Print("invitation retirement checkpoint pending")
			continue
		}
		if repo.ReleaseAddress(ctx, e, time.Now().UTC()) != nil {
			log.Print("invitation address release pending")
		}
	}
}
