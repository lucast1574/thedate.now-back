package httpapi

type wazendEvent struct {
	ID      string `json:"id"`
	Session string `json:"session"`
	Event   string `json:"event"`
	Payload struct {
		Vote struct {
			SelectedOptions []string `json:"selectedOptions"`
			Timestamp       int64    `json:"timestamp"`
		} `json:"vote"`
		Poll struct {
			ID     string `json:"id"`
			FromMe bool   `json:"fromMe"`
		} `json:"poll"`
		EventCreationKey struct {
			ID string `json:"id"`
		} `json:"eventCreationKey"`
		EventResponse struct {
			Response string `json:"response"`
		} `json:"eventResponse"`
	} `json:"payload"`
}

func (e wazendEvent) choice() (string, string, int64) {
	if e.Event == "poll.vote" && e.Payload.Poll.FromMe && len(e.Payload.Vote.SelectedOptions) == 1 {
		return map[string]string{"Sí": "going", "No": "not_going", "Tal vez": "maybe"}[e.Payload.Vote.SelectedOptions[0]], e.Payload.Poll.ID, e.Payload.Vote.Timestamp
	}
	if e.Event == "event.response" {
		return map[string]string{"GOING": "going", "NOT_GOING": "not_going", "MAYBE": "maybe"}[e.Payload.EventResponse.Response], e.Payload.EventCreationKey.ID, 0
	}
	return "", "", 0
}
