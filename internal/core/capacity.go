package core

import "time"

func IsReservationActive(g Guest, now time.Time) bool {
	if g.Response == "going" {
		return true
	}
	return g.Response == "maybe" && g.MaybeExpiresAt != nil && g.MaybeExpiresAt.After(now)
}

func ReservedSeats(guests []Guest, now time.Time) int {
	total := 0
	for _, g := range guests {
		if IsReservationActive(g, now) {
			total += g.Seats
		}
	}
	return total
}
