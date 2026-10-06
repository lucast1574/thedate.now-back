package dokploy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRendererReleaseRequiresValidRegistryDigest(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Error("manifest request must be HEAD")
		}
		w.Header().Set("Docker-Content-Digest", digest)
	}))
	defer server.Close()
	image, err := ResolveRenderer(context.Background(), server.Client(), "registry.example.test/renderer", server.URL)
	if err != nil || image != "registry.example.test/renderer@"+digest {
		t.Fatal("digest not resolved")
	}
	digest = "latest"
	if _, err = ResolveRenderer(context.Background(), server.Client(), "registry.example.test/renderer", server.URL); err == nil {
		t.Fatal("mutable or malformed digest accepted")
	}
}
