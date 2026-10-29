package httpapi

import (
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// WAHA serializes the same message with a different fromMe/chat prefix in
// event.response. Match the message key itself, retaining legacy stored IDs.
func wazendMessageID(id string) string {
	if len(id) == 0 || len(id) > 512 {
		return ""
	}
	parts := strings.SplitN(id, "_", 4)
	if len(parts) >= 3 && (parts[0] == "true" || parts[0] == "false") && strings.Contains(parts[1], "@") {
		return parts[2]
	}
	return id
}

func wazendMessageFilter(id string) bson.M {
	return bson.M{"$or": bson.A{
		bson.M{"invitationMessageId": id},
		bson.M{"invitationMessageId": bson.M{"$regex": "^(true|false)_[^_]+_" + regexp.QuoteMeta(id) + "(_[^_]+)?$"}},
	}}
}
