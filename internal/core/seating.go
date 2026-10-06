package core

import (
	"errors"
	"strconv"
	"strings"
)

type SeatingTable struct {
	ID       string  `bson:"id" json:"id"`
	Name     string  `bson:"name" json:"name"`
	Shape    string  `bson:"shape" json:"shape"`
	Capacity int     `bson:"capacity" json:"capacity"`
	X        float64 `bson:"x" json:"x"`
	Y        float64 `bson:"y" json:"y"`
	Rotation float64 `bson:"rotation" json:"rotation"`
}
type SeatingPlan struct {
	Version     int64             `bson:"version" json:"version"`
	Width       float64           `bson:"width" json:"width"`
	Height      float64           `bson:"height" json:"height"`
	Tables      []SeatingTable    `bson:"tables" json:"tables"`
	Assignments map[string]string `bson:"assignments" json:"assignments"`
}

func DefaultSeating() SeatingPlan {
	return SeatingPlan{Width: 1600, Height: 1000, Tables: []SeatingTable{}, Assignments: map[string]string{}}
}
func ValidateSeating(plan SeatingPlan, e Event, guests []Guest) error {
	if plan.Width != 1600 || plan.Height != 1000 || len(plan.Tables) > 100 || len(plan.Assignments) > 5000 || plan.Version < 0 {
		return errors.New("invalid plan size")
	}
	tables := map[string]int{}
	names := map[string]bool{}
	for _, t := range plan.Tables {
		name := strings.TrimSpace(t.Name)
		if t.ID == "" || len(t.ID) > 64 || tables[t.ID] != 0 || names[strings.ToLower(name)] || len(name) < 1 || len(name) > 60 || (t.Shape != "round" && t.Shape != "rectangle") || t.Capacity < 1 || t.Capacity > 50 || !bounded(t.X, 0, 1360) || !bounded(t.Y, 0, 800) || !bounded(t.Rotation, -180, 180) {
			return errors.New("invalid or duplicate table")
		}
		tables[t.ID] = t.Capacity
		names[strings.ToLower(name)] = true
	}
	allowed := map[string]bool{}
	for _, g := range guests {
		g = WithResponse(e, g)
		if g.Response == "not_going" {
			continue
		}
		for _, m := range PartyMembers(g) {
			allowed[m.ID] = true
		}
	}
	used := map[string]int{}
	for person, table := range plan.Assignments {
		if !allowed[person] || tables[table] == 0 {
			return errors.New("unknown attendee or table")
		}
		used[table]++
		if used[table] > tables[table] {
			return errors.New("table capacity exceeded")
		}
	}
	if !e.CapacityUnlimited && len(plan.Assignments) > e.Capacity {
		return errors.New("event capacity exceeded")
	}
	return nil
}
func ReconcileSeats(plan SeatingPlan, guestID string, response Response) map[string]string {
	assignments := map[string]string{}
	allowed := map[string]bool{}
	for n := 0; n < response.Seats; n++ {
		allowed[guestID+"~"+strconv.Itoa(n)] = response.Choice != "not_going"
	}
	for id, table := range plan.Assignments {
		if !strings.HasPrefix(id, guestID+"~") || allowed[id] {
			assignments[id] = table
		}
	}
	return assignments
}
