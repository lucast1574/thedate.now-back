package core

import (
	"errors"
	"time"
	_ "time/tzdata" // The production image does not install a zoneinfo database.
)

var ErrEventClosed = errors.New("event is closed")

// EventClosingAt is the next local midnight, including days with DST changes.
func EventClosingAt(e Event) time.Time {
	if e.IsDemo || e.StartAt.IsZero() {
		return time.Time{}
	}
	location, err := time.LoadLocation(e.TimeZone)
	if err != nil || e.TimeZone == "" {
		location = time.UTC
	}
	day := e.StartAt.In(location)
	return time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, location)
}

func EventClosed(e Event, now time.Time) bool {
	if e.DeletionRequested {
		return true
	}
	end := EventClosingAt(e)
	return !end.IsZero() && !now.Before(end)
}
