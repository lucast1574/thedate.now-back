package mongostore

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

type Responses struct{ DB *mongo.Database }

// Snapshot lazily migrates legacy guest responses with a single conditional write.
// Canonical responses live in the event, allowing atomic capacity checks on standalone MongoDB.
func (s Responses) Snapshot(ctx context.Context, id string) (core.Event, error) {
	var e core.Event
	err := s.DB.Collection("events").FindOne(ctx, bson.M{"_id": id}).Decode(&e)
	if err != nil || e.ResponseVersion > 0 {
		return e, err
	}
	cur, err := s.DB.Collection("guests").Find(ctx, bson.M{"eventId": id})
	if err != nil {
		return e, err
	}
	defer cur.Close(ctx)
	var guests []core.Guest
	if err = cur.All(ctx, &guests); err != nil {
		return e, err
	}
	responses := map[string]core.Response{}
	for _, g := range guests {
		if g.Response == "pending" {
			continue
		}
		r := core.Response{Seats: g.Seats, Choice: g.Response, Reason: g.MaybeReason, ExpiresAt: g.MaybeExpiresAt}
		if g.RespondedAt != nil {
			r.RespondedAt = *g.RespondedAt
		}
		responses[g.ID] = r
	}
	_, err = s.DB.Collection("events").UpdateOne(ctx, bson.M{"_id": id, "responseVersion": bson.M{"$in": bson.A{nil, 0}}}, bson.M{"$set": bson.M{"responses": responses, "responseVersion": 1, "responseReceipts": bson.M{}}})
	if err != nil {
		return e, err
	}
	err = s.DB.Collection("events").FindOne(ctx, bson.M{"_id": id}).Decode(&e)
	return e, err
}
func (s Responses) CommitResponse(ctx context.Context, e core.Event, id string, r core.Response, receipt string) (bool, error) {
	return s.commitResponse(ctx, e, id, r, receipt, true)
}
func (s Responses) CommitManagedResponse(ctx context.Context, e core.Event, id string, r core.Response) (bool, error) {
	return s.commitResponse(ctx, e, id, r, "", false)
}
func (s Responses) commitResponse(ctx context.Context, e core.Event, id string, r core.Response, receipt string, public bool) (bool, error) {
	set := bson.M{"responses." + id: r}
	increments := bson.M{"responseVersion": 1}
	cur, err := s.DB.Collection("guests").Find(ctx, bson.M{"eventId": e.ID})
	if err != nil {
		return false, err
	}
	defer cur.Close(ctx)
	var guests []core.Guest
	if err = cur.All(ctx, &guests); err != nil {
		return false, err
	}
	if len(guests) > 0 || e.Seating != nil {
		plan := core.SeatResponse(e, guests, id, r)
		// Legacy callers without stored metadata can still release obsolete seats.
		if len(guests) == 0 {
			plan.Assignments = core.ReconcileSeats(*e.Seating, id, r)
		}
		plan.Version++
		set["seating"] = plan
	}
	if receipt != "" {
		set["responseReceipts."+receipt] = true
	}
	filter := bson.M{"_id": e.ID, "responseVersion": e.ResponseVersion, "paymentStatus": "paid"}
	if public {
		filter["publishedAt"] = bson.M{"$ne": nil}
	}
	result, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": set, "$inc": increments, "$unset": bson.M{"invitationHolds." + id: ""}})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}
func (s Responses) CommitEvent(ctx context.Context, e core.Event, in core.EventInput, now time.Time) (bool, error) {
	set := bson.M{"invitationMessage": in.InvitationMessage, "title": in.Title, "description": in.Description, "startAt": in.StartAt, "timeZone": in.TimeZone, "organizer": in.Organizer, "location": in.Location, "isVirtual": in.IsVirtual, "mapUrl": in.MapURL, "virtualUrl": in.VirtualURL, "capacity": in.Capacity, "capacityUnlimited": in.CapacityUnlimited, "maybeHoldHours": in.MaybeHoldHours, "template": in.Template, "templateId": in.TemplateID, "designMode": in.DesignMode, "accentColor": in.AccentColor, "updatedAt": now}
	if in.Celebration != nil {
		set["celebration"] = in.Celebration
	}
	filter := bson.M{"_id": e.ID, "responseVersion": e.ResponseVersion}
	if in.Slug != e.Slug {
		set["slug"] = in.Slug
		filter["publishedAt"] = nil
		filter["invitationsEnabled"] = bson.M{"$ne": true}
	}
	result, err := s.DB.Collection("events").UpdateOne(ctx, filter, bson.M{"$set": set, "$inc": bson.M{"responseVersion": 1}})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}
