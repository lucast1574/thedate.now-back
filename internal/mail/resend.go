package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// SendResend uses a stable operation key so ambiguous responses can be retried safely.
func SendResend(ctx context.Context, c Config, to, subject, html, text, key string) Result {
	if !c.Valid() || c.APIKey == "" || strings.ContainsAny(to+subject, "\r\n") {
		return Result{"failed"}
	}
	payload, err := json.Marshal(map[string]any{"from": c.From, "to": []string{to}, "subject": subject, "html": html, "text": text})
	if err != nil {
		return Result{"failed"}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return Result{"failed"}
	}
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return Result{"uncertain"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Result{"failed"}
	}
	var accepted struct {
		ID string `json:"id"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&accepted) != nil || accepted.ID == "" {
		return Result{"uncertain"}
	}
	return Result{"sent"}
}
