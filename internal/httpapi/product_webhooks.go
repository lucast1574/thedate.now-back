package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type webhookResult struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *webhookResult) Header() http.Header  { return w.header }
func (w *webhookResult) WriteHeader(code int) { w.status = code }
func (w *webhookResult) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.body.Write(data)
}

func sharedWebhook(servers []*server, provider string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			bad(w, 400, "Invalid webhook body")
			return
		}
		var handled, failure *webhookResult
		for _, s := range servers {
			request := r.Clone(r.Context())
			request.Body = io.NopCloser(bytes.NewReader(body))
			result := &webhookResult{header: make(http.Header)}
			if provider == "stripe" {
				s.stripeWebhook(result, request)
			} else {
				s.wazendWebhook(result, request)
			}
			var payload map[string]string
			json.Unmarshal(result.body.Bytes(), &payload)
			if result.status >= 400 {
				if failure == nil || result.status >= 500 {
					failure = result
				}
			} else if status := payload["status"]; status != "ignored" && status != "unmatched" {
				handled = result
			}
		}
		if handled != nil {
			reply(w, 200, map[string]string{"status": "processed"})
			return
		}
		if failure != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(failure.status)
			w.Write(failure.body.Bytes())
			return
		}
		reply(w, 200, map[string]string{"status": "ignored"})
	})
}
