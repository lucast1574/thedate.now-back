package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Resolve WhatsApp's hidden sender identifier using the authenticated session.
func resolveMessageSender(ctx context.Context, config wazendSettings, from string) (string, error) {
	if !strings.HasSuffix(from, "@lid") {
		return from, nil
	}
	base, err := url.Parse(config.Base)
	if err != nil || base.Scheme != "https" || !config.ready() {
		return "", errors.New("sender resolution unavailable")
	}
	endpoint := strings.TrimSuffix(config.Base, "/") + "/api/" + url.PathEscape(config.Session) + "/lids/" + url.PathEscape(from)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Api-Key", config.Key)
	response, err := wazendClient.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return "", errors.New("sender resolution unavailable")
	}
	var mapping struct {
		PN string `json:"pn"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&mapping) != nil || mapping.PN == "" {
		return "", errors.New("sender phone unavailable")
	}
	return mapping.PN, nil
}
