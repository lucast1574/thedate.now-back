package dokploy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// The manifest URL is operator configuration, never user-controlled input.
func ResolveRenderer(ctx context.Context, client *http.Client, repository, manifest string) (string, error) {
	u, err := url.Parse(manifest)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !ValidImage(repository+"@sha256:"+"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		return "", fmt.Errorf("invalid renderer release configuration")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, manifest, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("renderer registry unavailable")
	}
	defer resp.Body.Close()
	image := repository + "@" + resp.Header.Get("Docker-Content-Digest")
	if resp.StatusCode != 200 || !ValidImage(image) {
		return "", fmt.Errorf("renderer manifest unavailable")
	}
	return image, nil
}
