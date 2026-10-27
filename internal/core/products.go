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
		return Product{Kind: kind, Role: "planner", Domain: "save.thedate.now", Label: "Save the Date - boda", PriceCents: 2500, AllowVirtual: true, AllowCouples: true}, nil
	case "general":
		return Product{Kind: kind, Role: "organizer", Domain: "thedate.now", Label: "The Date - evento", PriceCents: 500, AllowVirtual: true, AllowCouples: true}, nil
	default:
		return Product{}, errors.New("unknown event kind")
	}
}

func UserCanCreate(kind string, u User) bool {
	return (u.Product == "" || u.Product == kind) && ValidProductRole(kind, u.Role)
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

func ValidProductRole(kind, role string) bool {
	p, err := ProductFor(kind)
	return err == nil && (role == "admin" || role == "user" || role == p.Role)
}
func CanShareEvent(kind string, u User) bool {
	p, err := ProductFor(kind)
	return err == nil && (u.Role == "admin" || u.Role == p.Role)
}
