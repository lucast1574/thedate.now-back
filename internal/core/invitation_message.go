package core

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// InvitationMessage places the personal RSVP link without touching public design copy.
func InvitationMessage(message, link string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return link
	}
	message = strings.ReplaceAll(message, "{{enlace}}", "{{link}}")
	if index := strings.Index(message, "{{link}}"); index >= 0 {
		before, after := message[:index], strings.ReplaceAll(message[index+8:], "{{link}}", "")
		if before != "" {
			last, _ := utf8.DecodeLastRuneInString(before)
			if !unicode.IsSpace(last) {
				before += " "
			}
		}
		if after != "" {
			first, _ := utf8.DecodeRuneInString(after)
			if unicode.IsLetter(first) || unicode.IsDigit(first) {
				after = " " + after
			}
		}
		return before + link + after
	}

	return message + "\n\n" + link
}
