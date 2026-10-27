package core

import "strings"

// InvitationMessage places the personal RSVP link without touching public design copy.
func InvitationMessage(message, link string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return link
	}
	if strings.Contains(message, "{{link}}") || strings.Contains(message, "{{enlace}}") {
		return strings.NewReplacer("{{link}}", link, "{{enlace}}", link).Replace(message)
	}
	return message + "\n\n" + link
}
