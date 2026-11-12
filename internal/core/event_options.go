package core

import "errors"

var ErrMaybeDisabled = errors.New("Esta invitación solo permite confirmar Sí o No.")

// Missing settings preserve the behavior of invitations created before this option.
func MaybeAllowed(e Event) bool { return e.MaybeEnabled == nil || *e.MaybeEnabled }

func GiftsEnabled(e Event) bool {
	return e.Celebration == nil || e.Celebration.GiftsEnabled == nil || *e.Celebration.GiftsEnabled
}

func GiftListAvailable(e Event) bool { return GiftsEnabled(e) && len(e.Wishlist.Items) > 0 }

func ConfirmationFollowupNeeded(e Event, g Guest) bool {
	return GiftListAvailable(e) || (g.Seats > 1 && !e.Responses[g.ID].PartyRegistered)
}

// Gift availability is an operational setting, independent of visual publication.
func preserveGiftSetting(e Event, current *Celebration) Event {
	var enabled *bool
	if current != nil {
		enabled = current.GiftsEnabled
	}
	if e.Celebration != nil {
		copy := *e.Celebration
		copy.GiftsEnabled = enabled
		e.Celebration = &copy
	} else if enabled != nil {
		e.Celebration = &Celebration{GiftsEnabled: enabled}
	}
	return e
}
