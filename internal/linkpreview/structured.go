package linkpreview

import (
	"encoding/json"
	"strings"
)

// Product JSON-LD is common in storefronts that do not expose Open Graph images.
// This only reads embedded data; it never fetches URLs found in the document.
func productMetadata(raw string) Metadata {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return Metadata{}
	}
	return findProduct(value, 0)
}

func findProduct(value any, depth int) Metadata {
	if depth > 12 {
		return Metadata{}
	}
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			if found := findProduct(child, depth+1); found.Title != "" {
				return found
			}
		}
	case map[string]any:
		if isProduct(node["@type"]) {
			name, _ := node["name"].(string)
			description, _ := node["description"].(string)
			return Metadata{Title: name, Description: description, ImageURL: productImage(node["image"], 0)}
		}
		return findProduct(node["@graph"], depth+1)
	}
	return Metadata{}
}

func isProduct(value any) bool {
	switch kind := value.(type) {
	case string:
		return kind == "Product" || strings.TrimSuffix(kind, "/") == "https://schema.org/Product" || strings.TrimSuffix(kind, "/") == "http://schema.org/Product"
	case []any:
		for _, entry := range kind {
			if name, ok := entry.(string); ok && isProduct(name) {
				return true
			}
		}
	}
	return false
}

func productImage(value any, depth int) string {
	if depth > 6 {
		return ""
	}
	switch image := value.(type) {
	case string:
		return image
	case []any:
		for _, candidate := range image {
			if found := productImage(candidate, depth+1); found != "" {
				return found
			}
		}
	case map[string]any:
		if value, ok := image["url"].(string); ok {
			return value
		}
		value, _ := image["contentUrl"].(string)
		return value
	}
	return ""
}
