package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func postInvitationMessage(ctx context.Context, endpoint, key string, payload map[string]any) (string, string) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "failed"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", "failed"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", key)
	resp, err := wazendClient.Do(req)
	if err != nil {
		return "", "unknown"
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "unknown"
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return "", "failed"
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		lower := strings.ToLower(string(data))
		if strings.Contains(lower, "not on whatsapp") || strings.Contains(lower, "no lid found") {
			return "", "failed"
		}
		return "", "unknown"
	}
	id := extractMessageID(data)
	if id == "" {
		return "", "unknown"
	}
	return id, "sent"
}
