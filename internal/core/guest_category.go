package core

import "strings"

func GuestCategory(value string) (string, bool) {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "", true
	}
	key := func(s string) string {
		return strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n").Replace(strings.ToLower(s))
	}
	for _, category := range []string{"Familia del novio", "Familia de la novia", "Amigos del novio", "Amigos de la novia", "Amigos", "Compañeros de trabajo", "Compañeros de trabajo del novio", "Compañeros de trabajo de la novia", "Otros"} {
		if key(category) == key(value) {
			return category, true
		}
	}
	return "", false
}
