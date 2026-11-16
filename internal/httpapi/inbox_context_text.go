package httpapi

import (
	"context"
	"strings"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Legacy text prompts remain understandable without making the guest repeat a
// technical code. Unrecognized replies do not start a chain of new prompts.
func contextTextReply(ctx context.Context, coordinator *server, m core.InboxMessage, candidates []messageCandidate) (*core.InboxMessage, []messageCandidate) {
	if m.ReplyTo == "" {
		return nil, nil
	}
	var original core.InboxMessage
	err := coordinator.db.Collection("inboxMessages").FindOne(ctx, bson.M{"eventId": "", "session": m.Session, "phone": m.Phone, "$or": bson.A{bson.M{"clarification": m.ReplyTo}, bson.M{"contextPolls.id": m.ReplyTo}}}).Decode(&original)
	if err != nil {
		return nil, nil
	}
	normalize := func(value string) string { return strings.ToLower(strings.Join(strings.Fields(value), " ")) }
	matched := []messageCandidate{}
	for _, c := range candidates {
		if normalize(m.Text) == normalize(c.Event.Title) {
			matched = append(matched, c)
		}
	}
	return &original, matched
}
