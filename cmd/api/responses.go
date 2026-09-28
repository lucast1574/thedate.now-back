package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var errCapacity = errors.New("event capacity reached")

func (s *server) applyResponse(ctx context.Context, e core.Event, g core.Guest, response, reason string) error {
	lockValue, _ := s.locks.LoadOrStore(e.ID, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if err := s.db.Collection("guests").FindOne(ctx, bson.M{"_id": g.ID}).Decode(&g); err != nil {
		return err
	}
	now := time.Now().UTC()
	cur, err := s.db.Collection("guests").Find(ctx, bson.M{"eventId": e.ID})
	if err != nil {
		return err
	}
	var all []core.Guest
	if err = cur.All(ctx, &all); err != nil {
		cur.Close(ctx)
		return err
	}
	cur.Close(ctx)
	if response == "going" || response == "maybe" {
		occupied := core.ReservedSeats(all, now)
		if core.IsReservationActive(g, now) {
			occupied -= g.Seats
		}
		if occupied+g.Seats > e.Capacity {
			return errCapacity
		}
	}
	var expiry *time.Time
	if response == "maybe" {
		t := now.Add(time.Duration(e.MaybeHoldHours) * time.Hour)
		expiry = &t
	}
	_, err = s.db.Collection("guests").UpdateByID(ctx, g.ID, bson.M{"$set": bson.M{"response": response, "maybeReason": strings.TrimSpace(reason), "maybeExpiresAt": expiry, "respondedAt": now}})
	return err
}
