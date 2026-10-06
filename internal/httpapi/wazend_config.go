package httpapi

import "os"

type wazendSettings struct{ Base, Key, Session, HMAC string }

func wazendFor(kind string) wazendSettings {
	prefix := "WAZEND_GENERAL_"
	if kind == "wedding" {
		prefix = "WAZEND_WEDDING_"
	}
	return wazendSettings{Base: env(prefix+"BASE_URL", os.Getenv("WAZEND_BASE_URL")), Key: env(prefix+"API_KEY", os.Getenv("WAZEND_API_KEY")), Session: env(prefix+"SESSION", os.Getenv("WAZEND_SESSION")), HMAC: env(prefix+"WEBHOOK_HMAC", os.Getenv("WAZEND_WEBHOOK_HMAC"))}
}
func (c wazendSettings) ready() bool { return c.Base != "" && c.Key != "" && c.Session != "" }
func invitationBrand(kind string) string {
	if kind == "wedding" {
		return "Save the Date"
	}
	return "The Date"
}
