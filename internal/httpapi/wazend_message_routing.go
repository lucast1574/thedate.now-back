package httpapi

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"regexp"
	"strings"
	"time"
)

type messageCandidate struct {
	Server *server
	Event  core.Event
	Guest  core.Guest
}

func messageCandidates(ctx context.Context, peers []*server, m core.InboxMessage) ([]messageCandidate, error) {
	result := []messageCandidate{}
	for _, peer := range peers {
		cur, err := peer.db.Collection("guests").Find(ctx, bson.M{"phone": m.Phone, "sentAt": bson.M{"$ne": nil}, "sendState": bson.M{"$ne": "removing"}})
		if err != nil {
			return nil, err
		}
		var guests []core.Guest
		err = cur.All(ctx, &guests)
		cur.Close(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range guests {
			var e core.Event
			err = peer.db.Collection("events").FindOne(ctx, bson.M{"_id": g.EventID, "isDemo": bson.M{"$ne": true}, "deletionRequested": bson.M{"$ne": true}, "paymentStatus": "paid", "publishedAt": bson.M{"$ne": nil}, "addressReleasedAt": nil}).Decode(&e)
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					continue
				}
				return nil, err
			}
			config := wazendFor(e.Kind)
			if config.Session == m.Session {
				result = append(result, messageCandidate{peer, e, g})
			}
		}
	}
	return result, nil
}
func candidateMatches(ctx context.Context, c messageCandidate, m core.InboxMessage) bool {
	if regexp.MustCompile(`(?i)(?:^|\W)` + core.EventMessageCode(c.Event.ID) + `(?:$|\W)`).MatchString(m.Text) {
		return true
	}
	if m.ReplyTo == "" {
		return false
	}
	if wazendMessageID(c.Guest.InvitationMessageID) == m.ReplyTo || wazendMessageID(c.Guest.InvitationTextMessageID) == m.ReplyTo {
		return true
	}
	var original core.InboxMessage
	err := c.Server.db.Collection("inboxMessages").FindOne(ctx, bson.M{"session": m.Session, "providerId": bson.M{"$in": bson.A{m.ReplyTo, "true_" + strings.TrimPrefix(m.Phone, "+") + "@c.us_" + m.ReplyTo, "false_" + strings.TrimPrefix(m.Phone, "+") + "@c.us_" + m.ReplyTo}}, "eventId": c.Event.ID, "guestId": c.Guest.ID}).Decode(&original)
	return err == nil
}
func routeIncoming(ctx context.Context, peers []*server, m core.InboxMessage) error {
	candidates, err := messageCandidates(ctx, peers, m)
	if err != nil {
		return err
	}
	// Unknown senders never create event conversations or trigger automatic replies.
	if len(candidates) == 0 {
		return nil
	}
	for _, peer := range peers {
		old, err := (mongostore.Inbox{DB: peer.db}).Message(ctx, m.ID)
		if err == nil {
			if err = peer.ensureMessageMedia(ctx, old); err != nil {
				return err
			}
			if old.EventID == "" {
				return peer.requestMessageContext(ctx, old, candidates[0].Event.Kind)
			}
			for _, c := range candidates {
				if old.EventID == c.Event.ID && old.GuestID == c.Guest.ID && (old.ReplyTo != "" || strings.Contains(strings.ToUpper(old.Text), core.EventMessageCode(c.Event.ID))) {
					return resolvePendingMessage(ctx, peers[0], c, old)
				}
			}
			return nil
		}
	}
	matches := []messageCandidate{}
	for _, c := range candidates {
		if candidateMatches(ctx, c, m) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 && m.ReplyTo == "" && len(candidates) == 1 {
		matches = candidates
	}
	coordinator := peers[0]
	if len(matches) != 1 {
		m.EventID, m.GuestID = "", ""
		if err = coordinator.saveIncoming(ctx, &m); err != nil {
			return err
		}
		return coordinator.requestMessageContext(ctx, m, candidates[0].Event.Kind)
	}
	c := matches[0]
	m.EventID, m.GuestID = c.Event.ID, c.Guest.ID
	if err = c.Server.saveIncoming(ctx, &m); err != nil {
		return err
	}
	if m.ReplyTo != "" || strings.Contains(strings.ToUpper(m.Text), core.EventMessageCode(c.Event.ID)) {
		return resolvePendingMessage(ctx, coordinator, c, m)
	}
	return nil
}
func (s *server) saveIncoming(ctx context.Context, m *core.InboxMessage) error {
	repo := mongostore.Inbox{DB: s.db}
	old, err := repo.Message(ctx, m.ID)
	if err == nil {
		if old.EventID != m.EventID || old.GuestID != m.GuestID {
			return errors.New("message already associated")
		}
		return s.ensureMessageMedia(ctx, old)
	}
	if s.product == "wedding" {
		if m.Media != nil {
			m.Media.BucketKind = "wedding"
		}
	} else if m.Media != nil {
		m.Media.BucketKind = "general"
	}
	_, err = repo.Put(ctx, *m)
	if err != nil {
		return err
	}
	return s.ensureMessageMedia(ctx, *m)
}
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
