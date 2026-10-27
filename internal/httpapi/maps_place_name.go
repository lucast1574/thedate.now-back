package httpapi

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Use the place explicitly selected in the shared URL, never a nearby reverse-geocoding POI.
func extractMapsPlaceName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || !googleMapsURL(raw) {
		return ""
	}
	path := u.EscapedPath()
	marker := "/maps/place/"
	index := strings.Index(path, marker)
	if index < 0 {
		return ""
	}
	segment := strings.SplitN(path[index+len(marker):], "/", 2)[0]
	name, err := url.QueryUnescape(segment)
	if err != nil {
		return ""
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "@") || strings.ContainsAny(name, "<>") || utf8.RuneCountInString(name) > 240 {
		return ""
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return ""
		}
	}
	// Pin-only links carry coordinates instead of a venue name.
	if _, _, ok := extractMapsCoordinates("https://www.google.com/maps?q=" + url.QueryEscape(name)); ok {
		return ""
	}
	return name
}
func mapsDisplayAddress(name, address string) string {
	name, address = strings.TrimSpace(name), strings.TrimSpace(address)
	if name == "" {
		return address
	}
	lowerName, lowerAddress := strings.ToLower(name), strings.ToLower(address)
	if lowerAddress == lowerName || strings.HasPrefix(lowerAddress, lowerName+",") || strings.HasPrefix(lowerAddress, lowerName+" ·") {
		return address
	}
	if address == "" {
		return name
	}
	return name + " · " + address
}
