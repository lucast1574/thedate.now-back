package linkpreview

import (
	"bytes"
	"context"
	"errors"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

type Metadata struct {
	Title       string `json:"title"`
	ImageURL    string `json:"imageUrl"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

func truncate(value string, max int) string {
	value = strings.Join(strings.Fields(value), " ")
	for len(value) > max {
		_, n := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-n]
	}
	return value
}
func Parse(body io.Reader, base string) Metadata {
	out := Metadata{URL: base}
	z := html.NewTokenizer(body)
	title := false
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			break
		}
		token := z.Token()
		if kind == html.StartTagToken && token.Data == "body" {
			break
		}
		if kind == html.StartTagToken && token.Data == "title" {
			title = true
			continue
		}
		if kind == html.EndTagToken && token.Data == "title" {
			title = false
		}
		if title && kind == html.TextToken && out.Title == "" {
			out.Title = token.Data
		}
		if (kind != html.StartTagToken && kind != html.SelfClosingTagToken) || token.Data != "meta" {
			continue
		}
		key, value := "", ""
		for _, attr := range token.Attr {
			if attr.Key == "property" || attr.Key == "name" {
				key = strings.ToLower(attr.Val)
			}
			if attr.Key == "content" {
				value = attr.Val
			}
		}
		switch key {
		case "og:title":
			out.Title = value
		case "og:description":
			out.Description = value
		case "description":
			if out.Description == "" {
				out.Description = value
			}
		case "og:image:secure_url":
			out.ImageURL = value
		case "og:image", "twitter:image":
			if out.ImageURL == "" {
				out.ImageURL = value
			}
		}
	}
	if u, err := url.Parse(base); err == nil {
		if image, err := u.Parse(out.ImageURL); err == nil && out.ImageURL != "" && ValidURL(image.String()) {
			out.ImageURL = image.String()
		} else {
			out.ImageURL = ""
		}
	}
	out.Title = truncate(out.Title, 240)
	out.Description = truncate(out.Description, 1000)
	return out
}
func Fetch(ctx context.Context, raw string) (Metadata, error) {
	if !ValidURL(raw) {
		return Metadata{}, errors.New("Usa un enlace HTTPS público de la tienda.")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	req.Header.Set("User-Agent", "TheDate/1.0 (+https://thedate.now; product preview)")
	req.Header.Set("Accept", "text/html")
	client := Client()
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return Metadata{}, errors.New("La tienda no comparte una vista previa. Completa los datos manualmente.")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		return Metadata{}, errors.New("No pudimos leer el artículo. Puedes completar sus datos.")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return Metadata{}, errors.New("La página es demasiado grande para previsualizarla.")
	}
	data := Parse(strings.NewReader(string(body)), res.Request.URL.String())
	data.URL = raw
	return data, nil
}
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
