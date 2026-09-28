package core

import "time"

type User struct {
	ID           string    `bson:"_id" json:"id"`
	Email        string    `bson:"email" json:"email"`
	Name         string    `bson:"name" json:"name"`
	PasswordHash string    `bson:"passwordHash" json:"-"`
	Role         string    `bson:"role" json:"role"` // planner or organizer
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
}

type Event struct {
	ID             string     `bson:"_id" json:"id"`
	Kind           string     `bson:"kind" json:"kind"` // wedding or general
	Slug           string     `bson:"slug" json:"slug"`
	Title          string     `bson:"title" json:"title"`
	Description    string     `bson:"description" json:"description"`
	StartAt        time.Time  `bson:"startAt" json:"startAt"`
	Location       string     `bson:"location" json:"location"`
	Capacity       int        `bson:"capacity" json:"capacity"`
	MaybeHoldHours int        `bson:"maybeHoldHours" json:"maybeHoldHours"`
	Template       string     `bson:"template" json:"template"`
	AccentColor    string     `bson:"accentColor" json:"accentColor"`
	PhotoKeys      []string   `bson:"photoKeys" json:"photoKeys"`
	OwnerID        string     `bson:"ownerId" json:"ownerId"`
	CoupleUserIDs  []string   `bson:"coupleUserIds" json:"coupleUserIds"`
	PaymentStatus  string     `bson:"paymentStatus" json:"paymentStatus"` // unpaid, pending, paid
	CheckoutID     string     `bson:"checkoutId" json:"-"`
	PublishedAt    *time.Time `bson:"publishedAt" json:"publishedAt"`
	CreatedAt      time.Time  `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time  `bson:"updatedAt" json:"updatedAt"`
}

type Guest struct {
	ID                  string     `bson:"_id" json:"id"`
	EventID             string     `bson:"eventId" json:"eventId"`
	Name                string     `bson:"name" json:"name"`
	Phone               string     `bson:"phone" json:"phone"`
	Seats               int        `bson:"seats" json:"seats"`
	Response            string     `bson:"response" json:"response"` // pending, going, not_going, maybe
	MaybeReason         string     `bson:"maybeReason" json:"maybeReason"`
	MaybeExpiresAt      *time.Time `bson:"maybeExpiresAt" json:"maybeExpiresAt"`
	InviteToken         string     `bson:"inviteToken" json:"-"`
	InvitationMessageID string     `bson:"invitationMessageId" json:"-"`
	SentAt              *time.Time `bson:"sentAt" json:"sentAt"`
	RespondedAt         *time.Time `bson:"respondedAt" json:"respondedAt"`
	CreatedAt           time.Time  `bson:"createdAt" json:"createdAt"`
}

type CoupleInvite struct {
	ID        string    `bson:"_id" json:"id"`
	EventID   string    `bson:"eventId" json:"eventId"`
	Email     string    `bson:"email" json:"email"`
	TokenHash string    `bson:"tokenHash" json:"-"`
	ExpiresAt time.Time `bson:"expiresAt" json:"expiresAt"`
	Accepted  bool      `bson:"accepted" json:"accepted"`
}

type Receipt struct {
	ID        string    `bson:"_id" json:"id"`
	EventID   string    `bson:"eventId" json:"eventId"`
	Provider  string    `bson:"provider" json:"provider"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}
