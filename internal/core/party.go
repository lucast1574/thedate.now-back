package core

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

type Person struct {
	Name     string `bson:"name" json:"name"`
	LastName string `bson:"lastName" json:"lastName"`
	Gender   string `bson:"gender" json:"gender"`
}
type Member struct {
	ID, GuestID, Name, LastName, Gender, Family, Response string
	Slot                                                  int
	Registered                                            bool
}

func ValidPerson(p Person) bool {
	return len(strings.TrimSpace(p.Name)) >= 2 && len(p.Name) <= 120 && len(p.LastName) <= 80 && (p.Gender == "" || p.Gender == "unspecified" || p.Gender == "man" || p.Gender == "woman")
}
func ValidParty(people []Person, seats int) bool {
	if seats < 1 || seats > 20 || len(people) > seats-1 {
		return false
	}
	for _, p := range people {
		if !ValidPerson(p) {
			return false
		}
	}
	return true
}
func DecidePartyResponse(e Event, g Guest, choice, reason string, companions *[]Person, now time.Time) (Response, error) {
	selected := g.Companions
	seats := g.Seats
	registered := len(selected) == seats-1
	if previous, ok := e.Responses[g.ID]; ok && previous.PartyRegistered {
		selected = previous.Companions
		seats = previous.Seats
		registered = true
	}
	if companions != nil {
		selected = *companions
		seats = 1 + len(selected)
		registered = true
	}
	if !ValidParty(selected, g.Seats) {
		return Response{}, errors.New("invalid companion names or party size")
	}
	effective := g
	effective.Seats = seats
	next, err := DecideResponse(e, effective, choice, reason, now)
	if err != nil {
		return Response{}, err
	}
	next.Companions = selected
	next.PartyRegistered = registered
	return next, nil
}
func PartyMembers(g Guest) []Member {
	seats := g.Seats
	if g.AttendingSeats > 0 {
		seats = g.AttendingSeats
	}
	members := []Member{{ID: g.ID + "~0", GuestID: g.ID, Name: g.Name, LastName: g.LastName, Gender: g.Gender, Family: g.Family, Response: g.Response, Slot: 0, Registered: true}}
	for n := 1; n < seats; n++ {
		p := Person{Name: "Acompañante pendiente"}
		registered := false
		if n <= len(g.Companions) {
			p = g.Companions[n-1]
			registered = true
		}
		members = append(members, Member{ID: g.ID + "~" + strconv.Itoa(n), GuestID: g.ID, Name: p.Name, LastName: p.LastName, Gender: p.Gender, Family: g.Family, Response: g.Response, Slot: n, Registered: registered})
	}
	return members
}
