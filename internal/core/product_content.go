package core

import (
	"errors"
	"regexp"
	"strings"
)

var ErrWeddingContent = errors.New("para crear una boda utiliza Save the Date")
var weddingContent = regexp.MustCompile(`(?i)(?:^|[^\pL])(?:bodas?|casamiento|matrimonio|wedding|nos\s+casamos)(?:$|[^\pL])`)

// Product identity comes from the stored event, never from a design request.
// This catches explicit wedding intent, not every possible semantic disguise.
func ValidateProductContent(kind, title string, celebration *Celebration, sections []Section) error {
	if kind != "general" {
		return nil
	}
	if weddingContent.MatchString(title) || (celebration != nil && (strings.TrimSpace(celebration.PartnerOne) != "" || strings.TrimSpace(celebration.PartnerTwo) != "")) {
		return ErrWeddingContent
	}
	for _, section := range sections {
		if weddingContent.MatchString(section.Heading) || weddingContent.MatchString(section.Body) {
			return ErrWeddingContent
		}
		if section.Canvas == nil {
			continue
		}
		for _, item := range section.Canvas.Elements {
			if item.Binding == "couple_names" || (item.Type == "text" && item.Binding != "dress_code" && item.Binding != "restrictions" && weddingContent.MatchString(item.Text)) {
				return ErrWeddingContent
			}
		}
	}
	return nil
}
