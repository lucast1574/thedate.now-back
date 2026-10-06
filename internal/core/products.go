package core

import "errors"

type Product struct {
	Kind, Role, Domain, Label  string
	PriceCents                 int64
	AllowVirtual, AllowCouples bool
}

func ProductFor(kind string) (Product, error) {
	switch kind {
	case "wedding":
		return Product{Kind: kind, Role: "planner", Domain: "save.thedate.now", Label: "Save the Date - boda", PriceCents: 2500, AllowCouples: true}, nil
	case "general":
		return Product{Kind: kind, Role: "organizer", Domain: "thedate.now", Label: "The Date - evento", PriceCents: 500, AllowVirtual: true}, nil
	default:
		return Product{}, errors.New("unknown event kind")
	}
}

func UserCanCreate(kind string, u User) bool {
	if CanCreateEvent(kind, u.Role) {
		return true
	}
	if u.Role == "couple" {
		return false
	}
	if _, err := ProductFor(kind); err != nil {
		return false
	}
	for _, portal := range u.Portals {
		if portal == kind {
			return true
		}
	}
	return false
}
func UserPortals(u User) []string {
	portals := []string{}
	for _, kind := range []string{"wedding", "general"} {
		if UserCanCreate(kind, u) || (u.Role == "couple" && kind == "wedding") {
			portals = append(portals, kind)
		}
	}
	return portals
}
