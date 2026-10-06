package core

import "time"

type EventInput struct {
	TemplateID        string    `json:"templateId"`
	DesignMode        string    `json:"designMode"`
	CapacityUnlimited bool      `json:"capacityUnlimited"`
	Kind              string    `json:"kind"`
	Slug              string    `json:"slug"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	StartAt           time.Time `json:"startAt"`
	TimeZone          string    `json:"timeZone"`
	Organizer         string    `json:"organizer"`
	Location          string    `json:"location"`
	IsVirtual         bool      `json:"isVirtual"`
	MapURL            string    `json:"mapUrl"`
	VirtualURL        string    `json:"virtualUrl"`
	Capacity          int       `json:"capacity"`
	MaybeHoldHours    int       `json:"maybeHoldHours"`
	Template          string    `json:"template"`
	AccentColor       string    `json:"accentColor"`
}
