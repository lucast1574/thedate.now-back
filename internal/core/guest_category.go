package core

import "strings"

func GuestCategory(value string, kinds ...string) (string, bool) {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "", true
	}
	key := func(s string) string {
		return strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n").Replace(strings.ToLower(s))
	}
	categories := []string{"Familia del novio", "Familia de la novia", "Amigos del novio", "Amigos de la novia", "Amigos", "Compañeros de trabajo", "Compañeros de trabajo del novio", "Compañeros de trabajo de la novia", "Otros"}
	if len(kinds) > 0 && kinds[0] == "general" {
		categories = []string{"Familia", "Amigos", "Compañeros de trabajo", "Compañeros de estudios", "Vecinos", "Equipo o club", "Invitados especiales", "Otros"}
	}
	for _, category := range categories {
		if key(category) == key(value) {
			return category, true
		}
	}
	return "", false
}
