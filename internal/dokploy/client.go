package dokploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	URL, Key, APIURL string
	HTTP             *http.Client
}

var imagePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9./:_-]*@sha256:[a-f0-9]{64}$`)

func ValidImage(image string) bool { return imagePattern.MatchString(image) }
func New(base, key, apiURL string) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || key == "" {
		return nil, fmt.Errorf("invalid Dokploy configuration")
	}
	api, err := url.Parse(apiURL)
	if err != nil || api.Scheme != "https" || api.Host == "" || api.User != nil || strings.ContainsAny(apiURL, "\r\n") {
		return nil, fmt.Errorf("invalid public API URL")
	}
	return &Client{URL: strings.TrimSuffix(base, "/") + "/api", Key: key, APIURL: strings.TrimSuffix(apiURL, "/"), HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) call(ctx context.Context, endpoint string, input any, output any) error {
	method, target := http.MethodPost, c.URL+"/"+endpoint
	var body io.Reader
	if values, ok := input.(url.Values); ok {
		method = http.MethodGet
		target += "?" + values.Encode()
	} else {
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("Dokploy %s unavailable", endpoint)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Dokploy %s returned HTTP %d", endpoint, resp.StatusCode)
	}
	if output == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return err
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(output)
}
