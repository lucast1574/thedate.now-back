package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUnavailableObjectStorageDoesNotBlockAuthenticationStartup(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer upstream.Close()
	t.Setenv("S3_ENDPOINT", strings.TrimPrefix(upstream.URL, "http://"))
	t.Setenv("S3_ACCESS_KEY_ID", "test-access")
	t.Setenv("S3_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("S3_USE_SSL", "false")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	storage, err := newStorage(ctx)
	if err != nil || storage == nil || storage.client == nil {
		t.Fatal("unavailable media must not block the API", err)
	}
	if storage.bucket("wedding") == storage.bucket("general") {
		t.Fatal("product buckets mixed")
	}
}
