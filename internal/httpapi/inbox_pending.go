package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"time"
)

func resolvePendingMessage(ctx context.Context, coordinator *server, c messageCandidate, m core.InboxMessage) error {
	// A clarification can only resolve one outstanding message. Never move an
	// entire phone history into an event based on the latest reply.
	filter := bson.M{"eventId": bson.M{"$in": bson.A{"", "resolving:" + c.Event.ID}}, "session": m.Session, "phone": m.Phone, "receivedAt": bson.M{"$gte": m.ReceivedAt.Add(-24 * time.Hour), "$lte": m.ReceivedAt}}
	cur, err := coordinator.db.Collection("inboxMessages").Find(ctx, filter)
	if err != nil {
		return err
	}
	var pending []core.InboxMessage
	err = cur.All(ctx, &pending)
	cur.Close(ctx)
	if err != nil {
		return err
	}
	if len(pending) > 1 && m.ReplyTo != "" {
		specific := []core.InboxMessage{}
		for _, p := range pending {
			if wazendMessageID(p.Clarification) == m.ReplyTo {
				specific = append(specific, p)
			}
		}
		pending = specific
	}
	if len(pending) != 1 {
		return nil
	}
	p := pending[0]
	return associatePendingMessage(ctx, coordinator, c, p)
}

func associatePendingMessage(ctx context.Context, coordinator *server, c messageCandidate, p core.InboxMessage) error {
	p.EventID, p.GuestID = c.Event.ID, c.Guest.ID
	if coordinator.db.Name() == c.Server.db.Name() {
		seq, err := (mongostore.Inbox{DB: coordinator.db}).NextSequence(ctx, p.EventID, p.GuestID)
		if err != nil {
			return err
		}
		_, err = coordinator.db.Collection("inboxMessages").UpdateOne(ctx, bson.M{"_id": p.ID, "eventId": ""}, bson.M{"$set": bson.M{"eventId": p.EventID, "guestId": p.GuestID, "sequence": seq}})
		return err
	}
	// Claim before copying: a second clarification cannot expose the same photo.
	claim, err := coordinator.db.Collection("inboxMessages").UpdateOne(ctx, bson.M{"_id": p.ID, "eventId": bson.M{"$in": bson.A{"", "resolving:" + c.Event.ID}}}, bson.M{"$set": bson.M{"eventId": "resolving:" + c.Event.ID}})
	if err != nil {
		return err
	}
	if claim.MatchedCount == 0 {
		return nil
	}
	_, err = (mongostore.Inbox{DB: c.Server.db}).Put(ctx, p)
	if err != nil {
		return err
	}
	_, err = coordinator.db.Collection("inboxMessages").UpdateOne(ctx, bson.M{"_id": p.ID, "eventId": "resolving:" + c.Event.ID}, bson.M{"$set": bson.M{"eventId": "resolved", "guestId": ""}})
	return err
}
