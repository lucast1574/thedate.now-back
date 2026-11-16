package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const contextPageSize = 11
const moreContextEvents = "Ver más eventos"

func contextChoices(candidates []messageCandidate) []core.MessageContextChoice {
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Event.ID < candidates[j].Event.ID })
	choices := make([]core.MessageContextChoice, 0, len(candidates))
	for i, c := range candidates {
		e := c.Event
		zone, err := time.LoadLocation(e.TimeZone)
		if err != nil {
			zone = time.UTC
		}
		title := []rune(strings.Join(strings.Fields(e.Title), " "))
		if len(title) > 60 {
			title = title[:60]
		}
		label := fmt.Sprintf("%d. %s · %s · %s", i+1, string(title), e.StartAt.In(zone).Format("02/01/2006"), invitationBrand(e.Kind))
		choices = append(choices, core.MessageContextChoice{Label: label, EventID: e.ID, GuestID: c.Guest.ID, Kind: e.Kind})
	}
	return choices
}

func contextPage(choices []core.MessageContextChoice, page int) ([]core.MessageContextChoice, bool) {
	start := page * contextPageSize
	if page < 0 || start >= len(choices) {
		return nil, false
	}
	end := min(start+contextPageSize, len(choices))
	return choices[start:end], end < len(choices)
}

func (s *server) requestMessageContext(ctx context.Context, m core.InboxMessage, candidates []messageCandidate) error {
	if len(candidates) == 0 {
		return nil
	}
	return s.sendContextPage(ctx, m, contextChoices(candidates), candidates[0].Event.Kind, 0)
}

func (s *server) sendContextPage(ctx context.Context, m core.InboxMessage, choices []core.MessageContextChoice, kind string, page int) error {
	visible, more := contextPage(choices, page)
	if len(visible) == 0 {
		return nil
	}
	config := wazendFor(kind)
	base, err := url.Parse(config.Base)
	if err != nil || base.Scheme != "https" || !config.ready() {
		return nil
	}
	claim := fmt.Sprintf("contextPageClaims.%d", page)
	set := bson.M{claim: "sending"}
	if page == 0 {
		set["contextChoices"] = choices
	}
	result, err := s.db.Collection("inboxMessages").UpdateOne(ctx, bson.M{"_id": m.ID, "eventId": "", claim: bson.M{"$exists": false}}, bson.M{"$set": set})
	if err != nil || result.MatchedCount == 0 {
		return err
	}
	labels := make([]string, 0, len(visible)+1)
	for _, choice := range visible {
		labels = append(labels, choice.Label)
	}
	if more {
		labels = append(labels, moreContextEvents)
	}
	if len(labels) == 1 {
		labels = append(labels, "No corresponde a este evento")
	}
	payload := map[string]any{"session": config.Session, "chatId": strings.TrimPrefix(m.Phone, "+") + "@c.us", "reply_to": m.ProviderID, "poll": map[string]any{"name": "¿A qué evento corresponde lo que enviaste? Elige una opción.", "options": labels, "multipleAnswers": false}}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(config.Base, "/")+"/api/sendPoll", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", config.Key)
	req.Header.Set("Content-Type", "application/json")
	id := ""
	response, sendErr := wazendClient.Do(req)
	if sendErr == nil {
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if readErr == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
			id = extractMessageID(raw)
		}
	}
	// Persist the result even if the webhook request was cancelled. An unknown
	// send result is never retried blindly on provider redelivery.
	finish, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	update := bson.M{"$set": bson.M{claim: "uncertain"}}
	if id != "" {
		update = bson.M{"$set": bson.M{claim: "sent", "clarification": id}, "$push": bson.M{"contextPolls": core.MessageContextPoll{ID: id, Page: page}}}
	}
	_, err = s.db.Collection("inboxMessages").UpdateOne(finish, bson.M{"_id": m.ID, claim: "sending"}, update)
	return err
}
