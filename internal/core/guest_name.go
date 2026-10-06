package core

import "strings"

func GuestFullName(g Guest) string {
	return strings.TrimSpace(strings.TrimSpace(g.Name) + " " + strings.TrimSpace(g.LastName))
}
