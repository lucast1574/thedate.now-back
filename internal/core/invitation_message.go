package core

import "strings"

// InvitationMessage places the personal RSVP link at the top for WhatsApp preview without repeating it in the body.
func InvitationMessage(message, link string) string {
	clean := cleanInvitationMessage(message)
	if clean == "" {
		return "Abre tu invitación:\n" + link
	}
	return "Abre tu invitación:\n" + link + "\n\n" + clean
}

func cleanInvitationMessage(message string) string {
	for _, token := range []string{"{{link}}", "{{enlace}}", "{link}", "{enlace}"} {
		message = strings.ReplaceAll(message, token, "")
	}
	lines := strings.Split(message, "\n")
	var trimmed []string
	for _, line := range lines {
		trimmed = append(trimmed, strings.TrimRight(line, " \t\r"))
	}
	return strings.TrimSpace(strings.Join(trimmed, "\n"))
}
