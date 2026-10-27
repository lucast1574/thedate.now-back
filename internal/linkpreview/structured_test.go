package linkpreview

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestProductMetadataVariantsAndSafeImages(t *testing.T) {
	for _, image := range []string{`"/coffee.jpg"`, `["/coffee.jpg"]`, `{"@type":"ImageObject","url":"/coffee.jpg"}`, `{"contentUrl":"/coffee.jpg"}`} {
		source := fmt.Sprintf(`<head><title>Generic store</title><script type="application/ld+json">{"@graph":[{"@type":"BreadcrumbList"},{"@type":["Thing","Product"],"name":"Cafetera","image":%s}]}</script></head>`, image)
		data := Parse(strings.NewReader(source), "https://shop.example/item")
		if data.Title != "Cafetera" || data.ImageURL != "https://shop.example/coffee.jpg" {
			t.Fatal(data)
		}
	}
	for _, image := range []string{"https://127.0.0.1/a.png", "http://shop.example/a.jpg", "https://user:pass@shop.example/a.jpg"} {
		source := fmt.Sprintf(`<script type="application/ld+json">{"@type":"Product","name":"Gift","image":%q}</script>`, image)
		if data := Parse(strings.NewReader(source), "https://shop.example/item"); data.ImageURL != "" {
			t.Fatal("unsafe structured image accepted", data)
		}
	}
}

func TestMalformedStructuredDataKeepsOpenGraph(t *testing.T) {
	source := `<meta property="og:title" content="Known gift"><script type="application/ld+json">{broken</script><meta property="og:image" content="/safe.jpg">`
	data := Parse(strings.NewReader(source), "https://shop.example/item")
	if data.Title != "Known gift" || data.ImageURL != "https://shop.example/safe.jpg" {
		t.Fatal(data)
	}
}

func TestLargeStorefrontPreservesMetadataWithinBoundedPrefix(t *testing.T) {
	source := `<head><meta property="og:title" content="Readable gift"></head><body>` + strings.Repeat("x", 2<<20)
	data := Parse(io.LimitReader(strings.NewReader(source), 1<<20), "https://shop.example/item")
	if data.Title != "Readable gift" {
		t.Fatal(data)
	}
}
