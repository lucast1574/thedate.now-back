package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"os"
)

// Roll out readable links only after all invitation renderers serve the routes.
func guestInvitationPath(g core.Guest) string {
	if os.Getenv("PERSONAL_INVITATION_LINKS") != "true" {
		return "/rsvp/" + g.InviteToken
	}
	return core.GuestInvitationPath(g)
}
