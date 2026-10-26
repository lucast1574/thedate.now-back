package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	email "github.com/lucast1574/thedate.now-back/internal/mail"
	"go.mongodb.org/mongo-driver/v2/bson"
	"strings"
	"time"
)

func studioFor(kind string) string {
	if kind == "wedding" {
		return strings.TrimRight(env("WEDDING_STUDIO_URL", "https://studio.save.thedate.now"), "/")
	}
	return strings.TrimRight(env("EVENT_STUDIO_URL", "https://crea.thedate.now"), "/")
}
func (s *server) isEventMember(ctx context.Context, e core.Event, address string) bool {
	var u core.User
	if s.db.Collection("users").FindOne(ctx, bson.M{"email": address}).Decode(&u) != nil {
		return false
	}
	if u.ID == e.OwnerID {
		return true
	}
	for _, id := range e.CoupleUserIDs {
		if id == u.ID {
			return true
		}
	}
	return false
}
func (s *server) deliverAccess(ctx context.Context, e core.Event, i core.CoupleInvite, token, inviter string) string {
	content, err := email.Render(e.Kind, "invitation", email.TemplateData{Inviter: inviter, Title: e.Title, ActionURL: studioFor(e.Kind) + "/join/" + token})
	if err != nil {
		return "failed"
	}
	config := resendConfig(e.Kind)
	brand := "The Date"
	if e.Kind == "wedding" {
		brand = "Save the Date"
	}
	result := email.SendResend(ctx, config, i.Email, brand+" · Invitación para colaborar", content.HTML, content.Text, "invite:"+i.TokenHash)
	// Invitation tokens remain subject to the existing one-use delivery policy.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := s.db.Collection("events").UpdateOne(saveCtx, bson.M{"_id": e.ID, "coupleInvites." + i.TokenHash + ".tokenHash": i.TokenHash}, bson.M{"$set": bson.M{"coupleInvites." + i.TokenHash + ".delivery": result.State}, "$inc": bson.M{"coupleVersion": 1}}); err != nil {
		return "uncertain"
	}
	return result.State
}
