package linkpreview

import (
	"bytes"
	"context"
	"errors"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
)

func Image(ctx context.Context, raw string) ([]byte, string, error) {
	if !ValidURL(raw) {
		return nil, "", errors.New("invalid image URL")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	req.Header.Set("User-Agent", "TheDate/1.0")
	client := Client()
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, "", errors.New("image unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if err != nil || len(body) > 2<<20 {
		return nil, "", errors.New("image too large")
	}
	mime := http.DetectContentType(body)
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" && mime != "image/gif" {
		return nil, "", errors.New("unsupported image")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 16000 || config.Height > 16000 || int64(config.Width)*int64(config.Height) > 40000000 {
		return nil, "", errors.New("image dimensions too large")
	}
	return body, mime, nil
}
