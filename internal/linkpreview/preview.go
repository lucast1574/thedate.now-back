package linkpreview

import (
	"bytes"
	"golang.org/x/net/html"
	"io"
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
	content, _ := io.ReadAll(io.LimitReader(body, 4<<20))
	z := html.NewTokenizer(bytes.NewReader(content))
	title := false
	structured := Metadata{}
	inBody := false
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			break
		}
		token := z.Token()
		if kind == html.StartTagToken && token.Data == "body" {
			inBody = true
		}
		if kind == html.StartTagToken && token.Data == "script" {
			for _, attr := range token.Attr {
				if attr.Key == "type" && strings.EqualFold(strings.TrimSpace(attr.Val), "application/ld+json") {
					if z.Next() == html.TextToken {
						if product := productMetadata(string(z.Text())); product.Title != "" && structured.Title == "" {
							structured = product
						}
					}
					break
				}
			}
		}
		if inBody {
			continue
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
		case "og:title", "twitter:title":
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
	if structured.Title == "" && (out.Title == "" || out.ImageURL == "") {
		structured = microdata(content)
	}
	if structured.Title != "" {
		out.Title = structured.Title
		if structured.ImageURL != "" {
			out.ImageURL = structured.ImageURL
		}
		if out.Description == "" {
			out.Description = structured.Description
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
