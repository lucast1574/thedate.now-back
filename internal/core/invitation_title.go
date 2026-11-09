package core

import (
	"golang.org/x/text/unicode/norm"
	"regexp"
	"strings"
	"unicode"
)

var joinedWeddingNames = regexp.MustCompile(`(\p{Ll})y(\p{Lu})`)
var nameWord = regexp.MustCompile(`\p{L}[\p{L}\p{M}]*`)
var weddingNameSeparator = regexp.MustCompile(`\s+[yY]\s+`)

func CelebrationNames(c *Celebration) *Celebration {
	if c == nil {
		return nil
	}
	next := *c
	next.PartnerOne = capitalizedNames(c.PartnerOne)
	next.PartnerTwo = capitalizedNames(c.PartnerTwo)
	return &next
}

// Wedding titles are names, including titles imported without spaces around y.
// Other event titles retain their authored capitalization.
func InvitationTitle(kind, title string, names ...*Celebration) string {
	if kind != "wedding" {
		return strings.TrimSpace(title)
	}
	if len(names) > 0 && names[0] != nil {
		c := names[0]
		if strings.TrimSpace(c.PartnerOne) != "" && strings.TrimSpace(c.PartnerTwo) != "" && nameKey(title) == nameKey(c.PartnerOne+" y "+c.PartnerTwo) {
			return capitalizedNames(c.PartnerOne) + " y " + capitalizedNames(c.PartnerTwo)
		}
	}
	if !joinedWeddingNames.MatchString(title) && !weddingNameSeparator.MatchString(title) {
		return strings.TrimSpace(title)
	}
	title = joinedWeddingNames.ReplaceAllString(title, "${1} y ${2}")
	return capitalizedNames(title)
}

func nameKey(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, norm.NFD.String(value))
}

func capitalizedNames(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	first := true
	return nameWord.ReplaceAllStringFunc(title, func(word string) string {
		word = strings.ToLower(word)
		connector := word == "y" || word == "e" || word == "de" || word == "del" || word == "la" || word == "las" || word == "los"
		if !first && connector {
			return word
		}
		first = false
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	})
}
