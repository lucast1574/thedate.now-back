package core

import "time"

type MessageMedia struct {
	Digest     string `bson:"digest,omitempty" json:"-"`
	URL        string `bson:"url,omitempty" json:"-"`
	Key        string `bson:"key,omitempty" json:"-"`
	BucketKind string `bson:"bucketKind,omitempty" json:"-"`
	MIME       string `bson:"mime" json:"mime"`
	Filename   string `bson:"filename" json:"filename"`
	Available  bool   `bson:"available" json:"available"`
}

type InboxMessage struct {
	PaymentRecorded bool          `bson:"-" json:"paymentRecorded,omitempty"`
	Sequence        int64         `bson:"sequence" json:"-"`
	ID              string        `bson:"_id" json:"id"`
	ProviderID      string        `bson:"providerId" json:"-"`
	Session         string        `bson:"session" json:"-"`
	Phone           string        `bson:"phone" json:"-"`
	EventID         string        `bson:"eventId" json:"eventId"`
	GuestID         string        `bson:"guestId" json:"guestId"`
	Direction       string        `bson:"direction" json:"direction"`
	Text            string        `bson:"text" json:"text"`
	Media           *MessageMedia `bson:"media,omitempty" json:"media,omitempty"`
	CreatedAt       time.Time     `bson:"createdAt" json:"createdAt"`
	ReceivedAt      time.Time     `bson:"receivedAt" json:"-"`
	SentBy          string        `bson:"sentBy,omitempty" json:"sentBy,omitempty"`
	Delivery        string        `bson:"delivery" json:"delivery"`
	ReplyTo         string        `bson:"replyTo,omitempty" json:"-"`
	Clarification   string        `bson:"clarification,omitempty" json:"-"`
}

type ConversationState struct {
	ReadSequence     int64     `bson:"readSequence" json:"-"`
	AttendedSequence int64     `bson:"attendedSequence" json:"-"`
	ID               string    `bson:"_id" json:"-"`
	EventID          string    `bson:"eventId" json:"-"`
	GuestID          string    `bson:"guestId" json:"guestId"`
	ReadAt           time.Time `bson:"readAt" json:"readAt"`
	AttendedAt       time.Time `bson:"attendedAt" json:"attendedAt"`
}

type Conversation struct {
	GuestID string       `json:"guestId"`
	Last    InboxMessage `json:"last"`
	Unread  bool         `json:"unread"`
	Pending bool         `json:"pending"`
}
