package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Couples struct{ DB *mongo.Database }

func (s Couples) Snapshot(ctx context.Context, id string) (core.Event, error) {
	var e core.Event
	err := s.DB.Collection("events").FindOne(ctx, bson.M{"_id": id}).Decode(&e)
	if err != nil || e.CoupleVersion > 0 {
		return e, err
	}
	cur, err := s.DB.Collection("coupleInvites").Find(ctx, bson.M{"eventId": id})
	if err != nil {
		return e, err
	}
	defer cur.Close(ctx)
	var invites []core.CoupleInvite
	if err = cur.All(ctx, &invites); err != nil {
		return e, err
	}
	pending := map[string]core.CoupleInvite{}
	for _, invite := range invites {
		if !invite.Accepted {
			pending[invite.TokenHash] = invite
		}
	}
	_, err = s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": id, "coupleVersion": bson.M{"$in": bson.A{nil, 0}}}, bson.M{"$set": bson.M{"coupleInvites": pending, "coupleVersion": 1}})
	if err != nil {
		return e, err
	}
	err = s.DB.Collection("events").FindOne(ctx, bson.M{"_id": id}).Decode(&e)
	return e, err
}
func (s Couples) Save(ctx context.Context, e core.Event, ids []string, invites map[string]core.CoupleInvite) (bool, error) {
	result, err := s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": e.ID, "coupleVersion": e.CoupleVersion, "kind": bson.M{"$in": bson.A{"wedding", "general"}}, "paymentStatus": "paid"}, bson.M{"$set": bson.M{"coupleUserIds": ids, "coupleInvites": invites}, "$inc": bson.M{"coupleVersion": 1}})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}
func (s Couples) Invite(ctx context.Context, id, hash string) (core.Event, core.CoupleInvite, error) {
	var e core.Event
	err := s.DB.Collection("events").FindOne(ctx, bson.M{"coupleInvites." + hash + ".tokenHash": hash}).Decode(&e)
	if err == mongo.ErrNoDocuments {
		var old core.CoupleInvite
		if err = s.DB.Collection("coupleInvites").FindOne(ctx, bson.M{"tokenHash": hash}).Decode(&old); err != nil {
			return e, old, err
		}
		e, err = s.Snapshot(ctx, old.EventID)
	}
	return e, e.CoupleInvites[hash], err
}
