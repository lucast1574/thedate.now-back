package httpapi

import (
	"context"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/application"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// expireUnansweredInvitations finalizes invitations sent over 3 days ago without response,
// setting their response to not_going and releasing their capacity holds.
func (s *server) expireUnansweredInvitations(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	threeDaysAgo := time.Now().UTC().Add(-72 * time.Hour)
	cur, err := s.db.Collection("guests").Find(ctx,
		bson.M{
			"sentAt":   bson.M{"$ne": nil, "$lt": threeDaysAgo},
			"response": "pending",
		},
		options.Find().SetLimit(20),
	)
	if err != nil {
		return
	}
	defer cur.Close(ctx)

	var guests []core.Guest
	if cur.All(ctx, &guests) != nil {
		return
	}

	app := application.Responses{
		Repository: mongostore.Responses{DB: s.db},
		Now:        func() time.Time { return time.Now().UTC() },
	}

	for _, g := range guests {
		e, err := (mongostore.Responses{DB: s.db}).Snapshot(ctx, g.EventID)
		if err != nil || e.PaymentStatus != "paid" || e.PublishedAt == nil {
			continue
		}
		if r, ok := e.Responses[g.ID]; ok && r.Choice != "" && r.Choice != "pending" {
			_, _ = s.db.Collection("guests").UpdateOne(ctx,
				bson.M{"_id": g.ID},
				bson.M{"$set": bson.M{"response": r.Choice}},
			)
			continue
		}
		if err := app.FinalizeParty(ctx, e.ID, g, "not_going", "Sin respuesta tras 3 días", nil); err == nil {
			_, _ = s.db.Collection("guests").UpdateOne(ctx,
				bson.M{"_id": g.ID},
				bson.M{"$set": bson.M{
					"response":          "not_going",
					"responseUpdatedAt": time.Now().UTC(),
				}},
			)
		}
	}
}
