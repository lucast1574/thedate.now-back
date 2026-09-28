package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

var reservedSlugs = map[string]bool{
	"www": true, "api": true, "app": true, "admin": true, "backoffice": true,
	"save": true, "mail": true, "static": true, "assets": true, "support": true,
}

func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) || strings.Contains(slug, "--") || reservedSlugs[slug] {
		return errors.New("slug must be a valid, available DNS label")
	}
	return nil
}

func InvitationHost(kind, slug string) (string, error) {
	if err := ValidateSlug(slug); err != nil {
		return "", err
	}
	switch kind {
	case "wedding":
		return fmt.Sprintf("%s.save.thedate.now", slug), nil
	case "general":
		return fmt.Sprintf("%s.thedate.now", slug), nil
	default:
		return "", errors.New("unknown event kind")
	}
}

// EventFromHost accepts only one label above the matching product domain.
// It deliberately ignores X-Forwarded-Host; the reverse proxy must preserve Host.
func EventFromHost(rawHost string) (kind, slug string, ok bool) {
	host := strings.ToLower(strings.TrimSuffix(strings.Split(rawHost, ":")[0], "."))
	if strings.HasSuffix(host, ".save.thedate.now") {
		slug = strings.TrimSuffix(host, ".save.thedate.now")
		kind = "wedding"
	} else if strings.HasSuffix(host, ".thedate.now") {
		slug = strings.TrimSuffix(host, ".thedate.now")
		kind = "general"
	} else {
		return "", "", false
	}
	if ValidateSlug(slug) != nil {
		return "", "", false
	}
	return kind, slug, true
}
