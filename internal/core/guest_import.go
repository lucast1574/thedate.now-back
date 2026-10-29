package core

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
)

// One event/phone identity is shared by manual creation and Excel retries.
func GuestPhoneID(eventID, phone string) string {
	key := sha256.Sum256([]byte(eventID + ":" + phone))
	return "import-" + hex.EncodeToString(key[:])
}

// Reimport only edits descriptive data. RSVP, capacity, tokens and queue stay intact.
// Empty optional spreadsheet cells never erase richer data entered elsewhere.
func MergeGuestImport(existing, incoming Guest) (Guest, bool) {
	merged := existing
	merged.Name = strings.TrimSpace(incoming.Name)
	if value := strings.TrimSpace(incoming.LastName); value != "" {
		merged.LastName = value
	}
	if value := strings.TrimSpace(incoming.Family); value != "" {
		merged.Family = value
	}
	if incoming.Gender != "" && incoming.Gender != "unspecified" {
		merged.Gender = incoming.Gender
	}
	review := incoming.Seats != existing.Seats || (len(incoming.Companions) > 0 && !slices.Equal(incoming.Companions, existing.Companions))
	return merged, review
}
