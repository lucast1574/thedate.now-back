package mongostore

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"
)

var ErrReceivedGift = errors.New("Este invitado tiene un regalo recibido. Revisa ese regalo antes de eliminarlo.")

var ErrGuestSending = errors.New("El envío está en curso. Espera a que termine antes de eliminar al invitado.")

type GuestManagement struct{ Responses }

func (s GuestManagement) Remove(ctx context.Context, eventID, guestID string, now time.Time) error {
	var g core.Guest
	// Compete atomically with the sender's claim, never remove a message in flight.
	err := s.DB.Collection("guests").FindOneAndUpdate(ctx, bson.M{"_id": guestID, "eventId": eventID, "$or": bson.A{bson.M{"sendState": bson.M{"$nin": bson.A{"sending", "removing"}}}, bson.M{"sendState": "sending", "sendStartedAt": bson.M{"$lt": now.Add(-2 * time.Minute)}}, bson.M{"sendState": "removing", "removalStartedAt": bson.M{"$lt": now.Add(-2 * time.Minute)}}}}, bson.M{"$set": bson.M{"sendClaim": "guest-removal", "sendState": "removing", "removalStartedAt": now}}, options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&g)
	if err != nil {
		return ErrGuestSending
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		update := bson.M{"$set": bson.M{"sendState": g.SendState}}
		if g.SendClaim != "" {
			update["$set"].(bson.M)["sendClaim"] = g.SendClaim
		} else {
			update["$unset"] = bson.M{"sendClaim": ""}
		}
		_, _ = s.DB.Collection("guests").UpdateOne(rollbackCtx, bson.M{"_id": guestID, "sendClaim": "guest-removal", "sendState": "removing"}, update)
	}()
	for attempt := 0; attempt < 32; attempt++ {
		e, err := s.Snapshot(ctx, eventID)
		if err != nil {
			return err
		}
		for _, gift := range e.Wishlist.Items {
			if gift.GuestID == guestID && gift.Delivered {
				return ErrReceivedGift
			}
		}
		_, err = s.DB.Collection("deletedGuestBackups").UpdateOne(ctx, bson.M{"_id": g.InviteToken}, bson.M{"$setOnInsert": bson.M{"guest": g, "eventId": eventID, "response": e.Responses[guestID], "seating": e.Seating, "wishlist": e.Wishlist, "deletedAt": now}}, options.UpdateOne().SetUpsert(true))
		if err != nil {
			return err
		}
		var guests []core.Guest
		cur, err := s.DB.Collection("guests").Find(ctx, bson.M{"eventId": eventID})
		if err != nil {
			return err
		}
		err = cur.All(ctx, &guests)
		cur.Close(ctx)
		if err != nil {
			return err
		}
		plan := core.SeatResponse(e, guests, guestID, core.Response{Choice: "not_going"})
		plan.Version++
		list := e.Wishlist
		list.Items = append([]core.Gift(nil), list.Items...)
		list.Version++
		for i, item := range list.Items {
			if item.GuestID == guestID {
				list.Items[i].GuestID = ""
				list.Items[i].ReservedAt = nil
			}
		}
		filter := bson.M{"_id": eventID, "responseVersion": e.ResponseVersion, "wishlist.version": e.Wishlist.Version}
		if e.Wishlist.Version == 0 {
			filter["wishlist.version"] = bson.M{"$in": bson.A{nil, 0}}
		}
		result, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": bson.M{"revokedGuestTokens." + g.InviteToken: true, "seating": plan, "wishlist": list}, "$unset": bson.M{"responses." + guestID: "", "invitationHolds." + guestID: ""}, "$inc": bson.M{"responseVersion": 1}})
		if err != nil {
			return err
		}
		if result.MatchedCount == 0 {
			continue
		}
		committed = true
		_, err = s.DB.Collection("guests").DeleteOne(ctx, bson.M{"_id": guestID, "eventId": eventID, "inviteToken": g.InviteToken, "sendClaim": "guest-removal"})
		return err
	}
	return errors.New("La lista cambió. Intenta de nuevo.")
}
