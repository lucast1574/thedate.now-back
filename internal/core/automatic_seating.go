package core

import (
	"fmt"
	"strings"
)

// A permitted but unnamed +1 is not an actual table attendee yet.
func SeatingMembers(g Guest) []Member {
	members := []Member{}
	for _, member := range PartyMembers(g) {
		if member.Registered {
			members = append(members, member)
		}
	}
	return members
}

// ConfirmedAssignments removes legacy provisional seats without moving confirmed parties.
func ConfirmedAssignments(plan SeatingPlan, e Event, guests []Guest) map[string]string {
	allowed := map[string]bool{}
	for _, g := range guests {
		g = WithResponse(e, g)
		if g.Response == "going" {
			for _, m := range SeatingMembers(g) {
				allowed[m.ID] = true
			}
		}
	}
	result := map[string]string{}
	for id, table := range plan.Assignments {
		if allowed[id] {
			result[id] = table
		}
	}
	return result
}

// SeatResponse runs inside the same version fence as RSVP and manual plan writes.
// Only this party moves; other confirmed guests keep their manual choices.
func SeatResponse(e Event, guests []Guest, id string, response Response) SeatingPlan {
	plan := DefaultSeating()
	if e.Seating != nil {
		plan = *e.Seating
		plan.Tables = append([]SeatingTable{}, plan.Tables...)
	}
	// Copy the response map: callers may retry using the original snapshot.
	responses := make(map[string]Response, len(e.Responses)+1)
	for key, value := range e.Responses {
		responses[key] = value
	}
	responses[id] = response
	e.Responses = responses
	plan.Assignments = ConfirmedAssignments(plan, e, guests)
	if response.Choice != "going" {
		return plan
	}
	var guest Guest
	for _, g := range guests {
		if g.ID == id {
			guest = WithResponse(e, g)
			break
		}
	}
	if guest.ID == "" {
		return plan
	}
	party := SeatingMembers(guest)
	previous := plan.Assignments[id+"~0"]
	for _, member := range party {
		delete(plan.Assignments, member.ID)
	}
	if len(plan.Assignments)+len(party) > 5000 {
		return plan
	}
	loads := map[string]int{}
	categories := map[string]map[string]bool{}
	for _, g := range guests {
		for _, member := range SeatingMembers(WithResponse(e, g)) {
			if table := plan.Assignments[member.ID]; table != "" {
				loads[table]++
				if categories[table] == nil {
					categories[table] = map[string]bool{}
				}
				categories[table][strings.ToLower(strings.TrimSpace(g.Family))] = true
			}
		}
	}
	category := strings.ToLower(strings.TrimSpace(guest.Family))
	target := ""
	// Keep a manually chosen table when the whole updated party still fits.
	for _, table := range plan.Tables {
		if table.ID == previous && table.Capacity-loads[table.ID] >= len(party) {
			target = table.ID
		}
	}
	if target == "" {
		for _, table := range plan.Tables {
			if table.Capacity-loads[table.ID] >= len(party) && categories[table.ID][category] {
				target = table.ID
				break
			}
		}
	}
	if target == "" {
		// Empty manual tables are also available; never mix unrelated categories automatically.
		for _, table := range plan.Tables {
			if loads[table.ID] == 0 && table.Capacity >= len(party) {
				target = table.ID
				break
			}
		}
	}
	if target == "" {
		n := len(plan.Tables) + 1
		for {
			conflict := false
			for _, table := range plan.Tables {
				if table.Name == fmt.Sprintf("Mesa %d", n) || table.ID == fmt.Sprintf("auto-%d", n) {
					conflict = true
					break
				}
			}
			if !conflict {
				break
			}
			n++
		}
		target = fmt.Sprintf("auto-%d", n)
		capacity := 10
		if len(party) > capacity {
			capacity = len(party)
		}
		index := len(plan.Tables)
		plan.Tables = append(plan.Tables, SeatingTable{ID: target, Name: fmt.Sprintf("Mesa %d", n), Shape: "round", Capacity: capacity, X: float64(50 + (index%5)*300), Y: float64(50 + ((index/5)%4)*240)})
	}
	for _, member := range party {
		plan.Assignments[member.ID] = target
	}
	return plan
}
