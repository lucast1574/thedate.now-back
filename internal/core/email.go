package core

import "time"

type EmailPayload struct{ From, To, Subject, HTML, Text string }
type EmailDelivery struct {
	State      string        `bson:"state"`
	NextAt     time.Time     `bson:"nextAt"`
	Deadline   time.Time     `bson:"deadline"`
	Lease      string        `bson:"lease,omitempty"`
	LeaseUntil time.Time     `bson:"leaseUntil,omitempty"`
	Attempts   int           `bson:"attempts"`
	Payload    *EmailPayload `bson:"payload,omitempty"`
}

func PendingEmail(now time.Time) EmailDelivery {
	return EmailDelivery{State: "pending", NextAt: now, Deadline: now.Add(23 * time.Hour)}
}
