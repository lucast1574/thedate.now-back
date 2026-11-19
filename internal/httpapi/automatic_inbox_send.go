package httpapi

import (
	"context"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *server) sendAutomaticMessage(ctx context.Context, e core.Event, g core.Guest, config wazendSettings, kind string, payload map[string]any) string {
	now := time.Now().UTC()
	text, _ := payload["text"].(string)
	m := core.InboxMessage{
		ID: messageIdentity(e.ID, "automation:"+g.InviteToken+":"+kind), Automation: kind,
		EventID: e.ID, GuestID: g.ID, Session: config.Session, Phone: g.Phone,
		Direction: "out", Text: text, CreatedAt: now, ReceivedAt: now, Delivery: "sending",
	}
	inserted, err := (mongostore.Inbox{DB: s.db}).Put(ctx, m)
	if err != nil || !inserted {
		return "unknown"
	}
	id, outcome := postInvitationMessage(ctx, strings.TrimSuffix(config.Base, "/")+"/api/sendText", config.Key, payload)
	final, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	delivery := outcome
	if outcome == "unknown" {
		delivery = "uncertain"
	}
	_, err = s.db.Collection("inboxMessages").UpdateByID(final, m.ID, bson.M{"$set": bson.M{"providerId": id, "delivery": delivery}})
	if err != nil {
		return "unknown"
	}
	return outcome
}
