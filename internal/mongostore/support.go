package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SupportTickets struct{ DB *mongo.Database }
type SupportFilter struct{ RequesterID, State string }

func (f SupportFilter) bson() bson.M {
	filter := bson.M{}
	if f.RequesterID != "" {
		filter["requesterId"] = f.RequesterID
	}
	if f.State != "" {
		filter["state"] = f.State
	}
	return filter
}
func (s SupportTickets) Prior(ctx context.Context, owner, requestID string) (core.SupportTicket, error) {
	var ticket core.SupportTicket
	err := s.DB.Collection("supportTickets").FindOne(ctx, bson.M{"requesterId": owner, "requestId": requestID}, options.FindOne().SetProjection(bson.M{"images.data": 0})).Decode(&ticket)
	return ticket, err
}
func (s SupportTickets) Create(ctx context.Context, ticket core.SupportTicket) (core.SupportTicket, error) {
	_, err := s.DB.Collection("supportTickets").InsertOne(ctx, ticket)
	if mongo.IsDuplicateKeyError(err) {
		return s.Prior(ctx, ticket.RequesterID, ticket.RequestID)
	}
	return ticket, err
}
func (s SupportTickets) List(ctx context.Context, filter SupportFilter, page int64) ([]core.SupportTicket, int64, error) {
	total, err := s.DB.Collection("supportTickets").CountDocuments(ctx, filter.bson())
	if err != nil {
		return nil, 0, err
	}
	cur, err := s.DB.Collection("supportTickets").Find(ctx, filter.bson(), options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}}).SetSkip((page-1)*20).SetLimit(20).SetProjection(bson.M{"images.data": 0}))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	tickets := []core.SupportTicket{}
	err = cur.All(ctx, &tickets)
	return tickets, total, err
}
func (s SupportTickets) Get(ctx context.Context, id, owner string, images bool) (core.SupportTicket, error) {
	filter := bson.M{"_id": id}
	if owner != "" {
		filter["requesterId"] = owner
	}
	opts := options.FindOne()
	if !images {
		opts.SetProjection(bson.M{"images.data": 0})
	}
	var ticket core.SupportTicket
	err := s.DB.Collection("supportTickets").FindOne(ctx, filter, opts).Decode(&ticket)
	return ticket, err
}
func (s SupportTickets) Change(ctx context.Context, previous core.SupportTicket, change core.SupportChange) (core.SupportTicket, error) {
	set := bson.M{"state": change.State, "updatedAt": change.At}
	if change.State == "DONE" {
		set["resolvedAt"] = change.At
		set["mail.ticket_resolved"] = core.PendingEmail(change.At)
	}
	var ticket core.SupportTicket
	err := s.DB.Collection("supportTickets").FindOneAndUpdate(ctx, bson.M{"_id": previous.ID, "version": previous.Version, "state": previous.State}, bson.M{"$set": set, "$inc": bson.M{"version": 1}, "$push": bson.M{"history": change}}, options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(bson.M{"images.data": 0})).Decode(&ticket)
	return ticket, err
}
