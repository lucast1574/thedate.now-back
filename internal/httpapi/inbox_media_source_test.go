package httpapi

import "testing"

func TestStorageMediaIsRestrictedAndReceivesNoAPIKey(t *testing.T) {
	base := "https://storage.test/wazend-bucket"
	good := base + "/session/photo.webp?signature=private"
	t.Setenv("WAZEND_MEDIA_BASE_URL", base)
	if !trustedMessageMediaURL("https://wazend.test", good) {
		t.Fatal("configured storage rejected")
	}
	for _, raw := range []string{
		"http://storage.test/wazend-bucket/photo",
		"https://storage.test.evil/wazend-bucket/photo",
		"https://storage.test/other/photo",
		base + "-other/photo",
		base + "/../other/photo",
		base + "/%2e%2e/other/photo",
		"https://user@storage.test/wazend-bucket/photo",
	} {
		if trustedMessageMediaURL("https://wazend.test", raw) {
			t.Fatal("untrusted media accepted", raw)
		}
	}
	if messageMediaRequestHeader("https://wazend.test", good, "secret").Get("X-Api-Key") != "" {
		t.Fatal("API key leaked to storage")
	}
	if messageMediaRequestHeader("https://wazend.test", "https://wazend.test/api/files/photo", "secret").Get("X-Api-Key") != "secret" {
		t.Fatal("API media authentication missing")
	}
	t.Setenv("WAZEND_MEDIA_BASE_URL", "")
	if trustedMessageMediaURL("https://wazend.test", good) {
		t.Fatal("unconfigured storage accepted")
	}
}
