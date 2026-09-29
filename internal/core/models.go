package core

import "time"

type User struct {
	ID           string    `bson:"_id" json:"id"`
	Email        string    `bson:"email" json:"email"`
	Name         string    `bson:"name" json:"name"`
	PasswordHash string    `bson:"passwordHash" json:"-"`
	Role         string    `bson:"role" json:"role"` // admin, planner, organizer or couple
	GoogleSub    string    `bson:"googleSub,omitempty" json:"-"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
}

type Event struct {
	ID             string     `bson:"_id" json:"id"`
	Kind           string     `bson:"kind" json:"kind"` // wedding or general
	Slug           string     `bson:"slug" json:"slug"`
	Title          string     `bson:"title" json:"title"`
	Description    string     `bson:"description" json:"description"`
	StartAt        time.Time  `bson:"startAt" json:"startAt"`
	TimeZone       string     `bson:"timeZone" json:"timeZone"`
	Organizer      string     `bson:"organizer" json:"organizer"`
	Location       string     `bson:"location" json:"location"`
	IsVirtual      bool       `bson:"isVirtual" json:"isVirtual"`
	MapURL         string     `bson:"mapUrl" json:"mapUrl"`
	VirtualURL     string     `bson:"virtualUrl" json:"virtualUrl"`
	Capacity       int        `bson:"capacity" json:"capacity"`
	MaybeHoldHours int        `bson:"maybeHoldHours" json:"maybeHoldHours"`
	Template       string     `bson:"template" json:"template"`
	AccentColor    string     `bson:"accentColor" json:"accentColor"`
	IsDemo         bool       `bson:"isDemo,omitempty" json:"isDemo"`
	Sections       []Section  `bson:"sections,omitempty" json:"sections"`
	PhotoKeys      []string   `bson:"photoKeys" json:"photoKeys"`
	OwnerID        string     `bson:"ownerId" json:"ownerId"`
	CoupleUserIDs  []string   `bson:"coupleUserIds" json:"coupleUserIds"`
	PaymentStatus  string     `bson:"paymentStatus" json:"paymentStatus"` // unpaid, pending, paid
	CheckoutID     string     `bson:"checkoutId" json:"-"`
	PublishedAt    *time.Time `bson:"publishedAt" json:"publishedAt"`
	CreatedAt      time.Time  `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time  `bson:"updatedAt" json:"updatedAt"`
}

type Section struct {
	ID       string `bson:"id" json:"id"`
	Icon     string `bson:"icon" json:"icon"`
	Heading  string `bson:"heading" json:"heading"`
	Body     string `bson:"body" json:"body"`
	PhotoKey string `bson:"photoKey,omitempty" json:"photoKey,omitempty"`
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
