package mongostore

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"
)

type Inbox struct{ DB *mongo.Database }

func (s Inbox) Put(ctx context.Context, m core.InboxMessage) (bool, error) {
	if m.EventID != "" && m.GuestID != "" {
		seq, err := s.NextSequence(ctx, m.EventID, m.GuestID)
		if err != nil {
			return false, err
		}
		m.Sequence = seq
	}
	_, err := s.DB.Collection("inboxMessages").InsertOne(ctx, m)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}
func (s Inbox) NextSequence(ctx context.Context, event, guest string) (int64, error) {
	var state struct {
		Sequence int64 `bson:"sequence"`
	}
	err := s.DB.Collection("conversationStates").FindOneAndUpdate(ctx, bson.M{"_id": event + ":" + guest}, bson.M{"$inc": bson.M{"sequence": 1}, "$setOnInsert": bson.M{"eventId": event, "guestId": guest}}, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&state)
	return state.Sequence, err
}
func (s Inbox) Message(ctx context.Context, id string) (core.InboxMessage, error) {
	var m core.InboxMessage
	err := s.DB.Collection("inboxMessages").FindOne(ctx, bson.M{"_id": id}).Decode(&m)
	return m, err
}
func (s Inbox) Messages(ctx context.Context, event, guest, before string) ([]core.InboxMessage, error) {
	filter := bson.M{"eventId": event, "guestId": guest}
	if before != "" {
		m, err := s.Message(ctx, before)
		if err != nil || m.EventID != event || m.GuestID != guest {
			return nil, errors.New("invalid cursor")
		}
		filter["sequence"] = bson.M{"$lt": m.Sequence}
	}
	cur, err := s.DB.Collection("inboxMessages").Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "sequence", Value: -1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	list := []core.InboxMessage{}
	err = cur.All(ctx, &list)
	return list, err
}
func (s Inbox) Conversations(ctx context.Context, event string) ([]core.Conversation, error) {
	cur, err := s.DB.Collection("inboxMessages").Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"eventId": event, "$or": bson.A{bson.M{"direction": "in"}, bson.M{"automation": bson.M{"$exists": true}}}}}},
		{{Key: "$sort", Value: bson.D{{Key: "direction", Value: 1}, {Key: "sequence", Value: -1}}}},
		{{Key: "$group", Value: bson.M{"_id": "$guestId", "last": bson.M{"$first": "$$ROOT"}}}},
		{{Key: "$sort", Value: bson.M{"last.receivedAt": -1}}},
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		Last core.InboxMessage `bson:"last"`
	}
	if err = cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	states, err := s.DB.Collection("conversationStates").Find(ctx, bson.M{"eventId": event})
	if err != nil {
		return nil, err
	}
	defer states.Close(ctx)
	var records []core.ConversationState
	if err = states.All(ctx, &records); err != nil {
		return nil, err
	}
	byGuest := map[string]core.ConversationState{}
	for _, v := range records {
		byGuest[v.GuestID] = v
	}
	result := []core.Conversation{}
	for _, row := range rows {
		m := row.Last
		st := byGuest[m.GuestID]
		incoming := m.Direction == "in"
		result = append(result, core.Conversation{GuestID: m.GuestID, Last: m, Unread: incoming && m.Sequence > st.ReadSequence, Pending: incoming && m.Sequence > st.AttendedSequence})
	}
	return result, nil
}
func (s Inbox) Mark(ctx context.Context, event, guest, through string, attended *bool) error {
	m, err := s.Message(ctx, through)
	if err != nil || m.EventID != event || m.GuestID != guest || m.Direction != "in" {
		return errors.New("invalid message")
	}
	update := bson.M{"$max": bson.M{"readAt": m.ReceivedAt, "readSequence": m.Sequence}, "$setOnInsert": bson.M{"eventId": event, "guestId": guest}}
	if attended != nil {
		if *attended {
			update["$max"].(bson.M)["attendedAt"] = m.ReceivedAt
			update["$max"].(bson.M)["attendedSequence"] = m.Sequence
		} else {
			update["$set"] = bson.M{"attendedAt": time.Time{}, "attendedSequence": 0}
		}
	}
	_, err = s.DB.Collection("conversationStates").UpdateOne(ctx, bson.M{"_id": event + ":" + guest}, update, options.UpdateOne().SetUpsert(true))
	return err
}

func EnsureInboxIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("inboxMessages").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "eventId", Value: 1}, {Key: "guestId", Value: 1}, {Key: "sequence", Value: -1}}},
		{Keys: bson.D{{Key: "eventId", Value: 1}, {Key: "direction", Value: 1}, {Key: "sequence", Value: -1}}},
		{Keys: bson.D{{Key: "session", Value: 1}, {Key: "phone", Value: 1}, {Key: "eventId", Value: 1}}},
		{Keys: bson.D{{Key: "session", Value: 1}, {Key: "providerId", Value: 1}}},
		{Keys: bson.D{{Key: "session", Value: 1}, {Key: "contextPolls.id", Value: 1}}},
	})
	return err
}
