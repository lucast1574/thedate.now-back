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
		return Product{Kind: kind, Role: "organizer", Domain: "thedate.now", Label: "The Date - evento", PriceCents: 500, AllowVirtual: true, AllowCouples: true}, nil
	default:
		return Product{}, errors.New("unknown event kind")
	}
}

func UserCanCreate(kind string, u User) bool {
	return CanCreateEvent(kind, u.Role)
}
func UserPortals(u User) []string {
	result := []string{}
	for _, kind := range []string{"wedding", "general"} {
		if UserCanCreate(kind, u) {
			result = append(result, kind)
		}
	}
	return result
}
