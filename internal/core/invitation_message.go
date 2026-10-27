package core

import "strings"

// InvitationMessage places the personal RSVP link without touching public design copy.
func InvitationMessage(message, fallback, link string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return fallback
	}
	if strings.Contains(message, "{{enlace}}") {
		return strings.ReplaceAll(message, "{{enlace}}", link)
	}
	return message + "\n\nConfirma tu asistencia aquí: " + link
}
