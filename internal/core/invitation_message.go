package core

import "strings"

const PollInstruction = "👇 Por favor, responde en la encuesta de aquí abajo para confirmar tu asistencia."

// InvitationMessage places the personal RSVP link at the top for WhatsApp preview without repeating it in the body,
// and enforces answering the poll directly below the message.
func InvitationMessage(message, link string) string {
	clean := cleanInvitationMessage(message)
	prompt := PollInstruction
	if strings.Contains(strings.ToLower(clean), "encuesta") {
		prompt = ""
	}
	if clean == "" {
		return "Abre tu invitación:\n" + link + "\n\n" + prompt
	}
	if prompt != "" {
		return "Abre tu invitación:\n" + link + "\n\n" + clean + "\n\n" + prompt
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
