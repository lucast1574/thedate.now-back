package core

import (
	"errors"
	"strings"
)

func ValidateEvent(in EventInput, creating bool) error {
	if _, err := ProductFor(in.Kind); err != nil {
		return err
	}
	if creating && ValidateSlug(in.Slug) != nil {
		return errors.New("invalid slug")
	}
	if TemplateID(in.Kind, in.Template, in.DesignMode, in.TemplateID) == "" {
		return errors.New("template belongs to another product or mode")
	}
	if (!in.CapacityUnlimited && (in.Capacity < 1 || in.Capacity > 100000)) || in.StartAt.IsZero() || !ValidEventPlace(in) || !ValidDesign(DesignInput{Title: in.Title, Description: in.Description, Template: in.Template, AccentColor: in.AccentColor}, nil) {
		return errors.New("invalid event details")
	}
	if in.MaybeHoldHours < 0 || in.MaybeHoldHours > 168 {
		return errors.New("invalid reservation period")
	}
	return nil
}
func CanCreateEvent(kind, role string) bool {
	_, err := ProductFor(kind)
	return err == nil && ValidRole(role)
}
func NormalizeEvent(in EventInput) EventInput {
	in.TemplateID = TemplateID(in.Kind, in.Template, in.DesignMode, in.TemplateID)
	if in.CapacityUnlimited {
		in.Capacity = 0
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Organizer = strings.TrimSpace(in.Organizer)
	in.Location = EventLocation(in)
	in.MapURL = PhysicalMapURL(in)
	in.VirtualURL = VirtualEventURL(in)
	if in.TimeZone == "" {
		in.TimeZone = "UTC"
	}
	return in
}
