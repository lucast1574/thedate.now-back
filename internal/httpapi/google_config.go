package httpapi

import "os"

func googleClientID(portal string) string {
	general, wedding := os.Getenv("GOOGLE_GENERAL_CLIENT_ID"), os.Getenv("GOOGLE_WEDDING_CLIENT_ID")
	if general != "" || wedding != "" {
		switch portal {
		case "general":
			return general
		case "wedding":
			return wedding
		default:
			return ""
		}
	}
	if portal != "general" && portal != "wedding" {
		return ""
	}
	return os.Getenv("GOOGLE_CLIENT_ID")
}
