package core

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxSupportImageBytes = 1 << 20

type SupportImage struct {
	MIME string `bson:"mime" json:"-"`
	Data []byte `bson:"data" json:"-"`
}
type SupportChange struct {
	State   string    `bson:"state" json:"state"`
	Note    string    `bson:"note" json:"note,omitempty"`
	At      time.Time `bson:"at" json:"at"`
	ActorID string    `bson:"actorId" json:"-"`
}
type SupportTicket struct {
	Mail           map[string]EmailDelivery `bson:"mail,omitempty" json:"-"`
	ID             string                   `bson:"_id" json:"id"`
	RequesterID    string                   `bson:"requesterId" json:"-"`
	RequesterName  string                   `bson:"requesterName" json:"-"`
	RequesterEmail string                   `bson:"requesterEmail" json:"-"`
	RequestID      string                   `bson:"requestId" json:"-"`
	Title          string                   `bson:"title" json:"title"`
	Description    string                   `bson:"description" json:"description"`
	State          string                   `bson:"state" json:"state"`
	Version        int64                    `bson:"version" json:"version"`
	Images         []SupportImage           `bson:"images" json:"-"`
	History        []SupportChange          `bson:"history" json:"history"`
	CreatedAt      time.Time                `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time                `bson:"updatedAt" json:"updatedAt"`
	ResolvedAt     *time.Time               `bson:"resolvedAt,omitempty" json:"resolvedAt,omitempty"`
}

func ValidSupportState(state string) bool {
	return state == "OPEN" || state == "IN_PROGRESS" || state == "WAITING" || state == "DONE"
}
func SupportText(title, description string) (string, string, error) {
	title, description = strings.TrimSpace(title), strings.TrimSpace(description)
	if utf8.RuneCountInString(title) < 3 || utf8.RuneCountInString(title) > 140 || utf8.RuneCountInString(description) < 10 || utf8.RuneCountInString(description) > 10000 {
		return "", "", errors.New("use a title of 3–140 characters and a description of 10–10000 characters")
	}
	return title, description, nil
}
func SupportTransition(previous SupportTicket, state, note, actor string, now time.Time) (SupportChange, error) {
	note = strings.TrimSpace(note)
	if !ValidSupportState(state) || utf8.RuneCountInString(note) > 1500 || previous.State == "DONE" || len(previous.History) >= 100 {
		return SupportChange{}, errors.New("invalid support transition")
	}
	if state == previous.State && note == "" {
		return SupportChange{}, errors.New("choose another state or add a reply")
	}
	return SupportChange{State: state, Note: note, At: now, ActorID: actor}, nil
}
