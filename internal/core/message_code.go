package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func EventMessageCode(id string) string {
	hash := sha256.Sum256([]byte(id))
	return "TD-" + strings.ToUpper(hex.EncodeToString(hash[:5]))
}
