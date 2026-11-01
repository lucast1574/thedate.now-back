package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"log"
	"time"
)

type mobileSender interface {
	Send(context.Context, string, core.MobileChange) (bool, error)
}

func (s *server) mobileWorker(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	after := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.mobileSender == nil {
				continue
			}
			run, cancel := context.WithTimeout(ctx, 18*time.Second)
			devices, err := (mongostore.MobileDevices{DB: s.db}).Page(run, after, time.Now().UTC())
			if len(devices) == 0 {
				after = ""
			}
			for _, d := range devices {
				if run.Err() != nil {
					break
				}
				after = d.ID
				if e := s.deliverMobile(run, d); e != nil {
					err = e
				}
			}
			cancel()
			if err != nil {
				log.Printf("Mobile notification processing incomplete for product %s", s.product)
			}
		}
	}
}
func (s *server) deliverMobile(ctx context.Context, d core.MobileDevice) error {
	filter := bson.M{"kind": s.product, "paymentStatus": "paid", "isDemo": bson.M{"$ne": true}, "$or": bson.A{bson.M{"ownerId": d.UserID}, bson.M{"coupleUserIds": d.UserID}}}
	cur, err := s.db.Collection("events").Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)
	var events []core.Event
	if err = cur.All(ctx, &events); err != nil {
		return err
	}
	repo := mongostore.MobileDevices{DB: s.db}
	// Changes older than one day are refreshed in-app rather than replayed on a lock screen.
	since := d.RegisteredAt
	if cutoff := time.Now().UTC().Add(-24 * time.Hour); since.Before(cutoff) {
		since = cutoff
	}
	for _, e := range events {
		for _, change := range core.MobileChanges(e, since) {
			current, err := repo.Current(ctx, d, e.ID)
			if err != nil {
				return err
			}
			if !current {
				return nil
			}
			id, err := repo.Claim(ctx, d, change, time.Now().UTC())
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			if err != nil {
				return err
			}
			invalid, sendErr := s.mobileSender.Send(ctx, d.FID, change)
			state := "sent"
			if sendErr != nil {
				state = "uncertain"
			}
			if invalid {
				state = "invalid"
				if err := repo.RemoveBinding(ctx, d); err != nil {
					return err
				}
			}
			if err := repo.Finish(ctx, id, state); err != nil {
				return err
			}
			if sendErr != nil {
				return sendErr
			}
			if invalid {
				return nil
			}
		}
	}
	return nil
}
