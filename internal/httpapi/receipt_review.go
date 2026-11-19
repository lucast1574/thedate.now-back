package httpapi

import (
	"context"
	"strings"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *server) annotateReceipts(ctx context.Context, e core.Event, g core.Guest, messages []core.InboxMessage) error {
	recorded, digests := map[string]bool{}, map[string]bool{}
	for _, payment := range e.GuestPayments {
		if payment.MessageID != "" {
			recorded[payment.MessageID] = true
		}
		if payment.ReceiptDigest != "" {
			digests[payment.ReceiptDigest] = true
		}
	}
	quotes := []string{}
	balance := core.GuestPaymentBalance(e, g)
	for i := range messages {
		m := &messages[i]
		m.PaymentRecorded = recorded[m.ID] || (m.Media != nil && digests[m.Media.Digest])
		if m.PaymentRecorded || m.Direction != "in" || m.Media == nil {
			continue
		}
		if !strings.HasPrefix(m.Media.MIME, "image/") && m.Media.MIME != "application/pdf" {
			continue
		}
		if balance > 0 {
			m.ReceiptReview = "review"
		}
		if m.ReplyTo != "" {
			quotes = append(quotes, m.ReplyTo)
		}
	}
	if len(quotes) == 0 {
		return nil
	}
	filter := bson.M{"eventId": e.ID, "guestId": g.ID, "automation": "payment", "providerId": bson.M{"$in": quotes}, "direction": "out"}
	cur, err := s.db.Collection("inboxMessages").Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)
	paymentQuotes := map[string]string{}
	for cur.Next(ctx) {
		var m core.InboxMessage
		if err := cur.Decode(&m); err != nil {
			return err
		}
		paymentQuotes[m.ProviderID] = m.Session
	}
	for i := range messages {
		m := &messages[i]
		if session, ok := paymentQuotes[m.ReplyTo]; ok && session == m.Session && m.Direction == "in" && !m.PaymentRecorded && m.Media != nil && (strings.HasPrefix(m.Media.MIME, "image/") || m.Media.MIME == "application/pdf") {
			m.ReceiptReview = "possible"
		}
	}
	return cur.Err()
}
