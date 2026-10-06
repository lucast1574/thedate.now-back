package httpapi

import (
	"github.com/lucast1574/thedate.now-back/internal/core"
	"strings"
)

type guestInput struct {
	Name       string        `json:"name"`
	LastName   string        `json:"lastName"`
	Family     string        `json:"family"`
	Gender     string        `json:"gender"`
	Phone      string        `json:"phone"`
	Seats      int           `json:"seats"`
	Companions []core.Person `json:"companions"`
}

func validGuest(in guestInput) bool {
	return core.ValidPerson(core.Person{Name: in.Name, LastName: in.LastName, Gender: in.Gender}) && len(in.Family) <= 80 && phonePattern.MatchString(in.Phone) && core.ValidParty(in.Companions, in.Seats)
}
func guestFromInput(in guestInput, id, eventID, token string) core.Guest {
	return core.Guest{ID: id, EventID: eventID, Name: strings.TrimSpace(in.Name), LastName: strings.TrimSpace(in.LastName), Family: strings.TrimSpace(in.Family), Gender: in.Gender, Phone: in.Phone, Seats: in.Seats, Companions: in.Companions, Response: "pending", InviteToken: token}
}
