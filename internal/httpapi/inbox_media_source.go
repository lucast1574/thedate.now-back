package httpapi

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

// Storage URLs come only from authenticated Wazend responses. Their origin and
// bucket prefix must also be explicitly trusted by the operator.
func safeStorageMediaURL(base, raw string) bool {
	b, err := url.Parse(base)
	if err != nil || b.Scheme != "https" || b.Host == "" || b.User != nil || b.RawQuery != "" || b.Fragment != "" || b.Path == "" || b.Path == "/" {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == b.Host && u.User == nil && u.Fragment == "" && path.Clean(u.Path) == u.Path && strings.HasPrefix(u.Path, strings.TrimSuffix(b.Path, "/")+"/")
}

func trustedMessageMediaURL(apiBase, raw string) bool {
	return safeMessageMediaURL(apiBase, raw) || safeStorageMediaURL(os.Getenv("WAZEND_MEDIA_BASE_URL"), raw)
}

func messageMediaRequestHeader(apiBase, raw, key string) http.Header {
	header := make(http.Header)
	if safeMessageMediaURL(apiBase, raw) {
		header.Set("X-Api-Key", key)
	}
	return header
}
