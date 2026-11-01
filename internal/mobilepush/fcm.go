package mobilepush

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"io"
	"net/http"
	"time"
)

type Client struct {
	Project string
	HTTP    *http.Client
}

func New(ctx context.Context, raw []byte, expected string) (*Client, error) {
	var account struct {
		Project  string `json:"project_id"`
		Type     string `json:"type"`
		TokenURI string `json:"token_uri"`
	}
	if json.Unmarshal(raw, &account) != nil || account.Type != "service_account" || account.Project != expected || account.TokenURI != "https://oauth2.googleapis.com/token" {
		return nil, errors.New("invalid product FCM credentials")
	}
	credentials, err := google.CredentialsFromJSON(ctx, raw, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return nil, err
	}
	client := oauth2.NewClient(ctx, credentials.TokenSource)
	client.Timeout = 10 * time.Second
	return &Client{Project: expected, HTTP: client}, nil
}
func (c *Client) Send(ctx context.Context, fid string, change core.MobileChange) (bool, error) {
	body := "Tu lista de invitados tiene una nueva respuesta."
	title := "Nueva confirmación"
	if change.Screen == "gifts" {
		title = "Nuevo regalo reservado"
		body = "Hay una nueva reserva en tu lista de regalos."
	}
	payload := map[string]any{"message": map[string]any{"fid": fid, "notification": map[string]string{"title": title, "body": body}, "data": map[string]string{"eventId": change.EventID, "screen": change.Screen}, "android": map[string]any{"priority": "HIGH", "ttl": "86400s", "notification": map[string]string{"channel_id": "celebration_updates", "tag": change.ID}}, "apns": map[string]any{"headers": map[string]string{"apns-collapse-id": change.ID}, "payload": map[string]any{"aps": map[string]any{"sound": "default"}}}}}
	raw, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://fcm.googleapis.com/v1/projects/"+c.Project+"/messages:send", bytes.NewReader(raw))
	if err != nil {
		return false, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.HTTP.Do(request)
	if err != nil {
		return false, errors.New("FCM outcome unknown")
	}
	defer response.Body.Close()
	raw, err = io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return false, errors.New("FCM outcome unknown")
	}
	var result struct {
		Name  string `json:"name"`
		Error struct {
			Details []struct {
				Code string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &result)
	for _, detail := range result.Error.Details {
		if detail.Code == "UNREGISTERED" {
			return true, nil
		}
	}
	if response.StatusCode != 200 || result.Name == "" {
		return false, errors.New("FCM send rejected")
	}
	return false, nil
}
