package core

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed lucide-symbols.json
var lucideNames []byte
var lucideSymbols = func() map[string]bool {
	var names []string
	if err := json.Unmarshal(lucideNames, &names); err != nil {
		panic(err)
	}
	symbols := make(map[string]bool, len(names))
	for _, name := range names {
		symbols[name] = true
	}
	return symbols
}()

func validFlyerIcon(name string) bool {
	return sectionIcons[name] || (strings.HasPrefix(name, "lucide:") && lucideSymbols[strings.TrimPrefix(name, "lucide:")])
}
func validElementBinding(el FlyerElement) bool {
	switch el.Binding {
	case "":
	case "guest_name", "dress_code", "adults_notice":
		if el.Type != "text" {
			return false
		}
	case "gift_link":
		if el.Type != "text" || !ValidGiftLink(el.Link) {
			return false
		}
	case "location":
		if el.Type != "text" || !validLocationStyle(el.LocationStyle) || len(el.Address) > 500 || len(el.Link) > 2048 || (el.Link != "" && !GoogleMapsURL(el.Link)) {
			return false
		}
	default:
		return false
	}
	return (el.Binding == "gift_link" && el.LocationStyle == "" && el.Address == "") || el.Binding == "location" || (el.LocationStyle == "" && el.Address == "" && el.Link == "")
}

func validLocationStyle(style string) bool {
	switch style {
	case "icon", "label", "address", "icon-label", "icon-address", "label-address", "all":
		return true
	}
	return false
}
