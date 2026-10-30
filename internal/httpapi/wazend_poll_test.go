package httpapi

import (
	"encoding/json"
	"testing"
)

func TestPollVotesUseTheSameAttendanceChoicesAsLegacyEvents(t *testing.T) {
	for label, want := range map[string]string{"Sí": "going", "No": "not_going", "Tal vez": "maybe"} {
		var e wazendEvent
		body, _ := json.Marshal(map[string]any{"event": "poll.vote", "payload": map[string]any{"poll": map[string]any{"id": "true_123@c.us_ID", "fromMe": true}, "vote": map[string]any{"selectedOptions": []string{label}, "timestamp": 123}}})
		if err := json.Unmarshal(body, &e); err != nil {
			t.Fatal(err)
		}
		choice, id, stamp := e.choice()
		if choice != want || id != "true_123@c.us_ID" || stamp != 123 {
			t.Fatal("poll vote decoded incorrectly")
		}
		e.Payload.Poll.FromMe = false
		if c, _, _ := e.choice(); c != "" {
			t.Fatal("foreign poll accepted")
		}
	}
}
