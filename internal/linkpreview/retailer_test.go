package linkpreview

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSharedMercadoLinksKeepSelectedItem(t *testing.T) {
	raw := "https://www.mercadolibre.com.pe/up/MPEU3667914835?matt_tool=56836481&pdp_filters=item_id:MPE913032834&ua=tracker#origin=share&wid=MPE913032834"
	u, _ := url.Parse(NormalizeURL(raw))
	if u.Query().Get("pdp_filters") != "item_id:MPE913032834" || u.Query().Has("ua") || u.Query().Has("matt_tool") || u.Fragment != "" {
		t.Fatal(u)
	}
	u, _ = url.Parse(NormalizeURL("https://www.mercadolibre.com.mx/up/MLMU123#origin=share&wid=MLM12345"))
	if u.Query().Get("pdp_filters") != "item_id:MLM12345" {
		t.Fatal(u)
	}
	u, _ = url.Parse(NormalizeURL("https://shop.example/products/cup?variant=42&utm_source=share"))
	if u.Query().Get("variant") != "42" || u.Query().Has("utm_source") {
		t.Fatal(u)
	}
	if mercadoHost("mercadolibre.com.pe.evil.test") {
		t.Fatal("hostname boundary")
	}
}
func TestProductDataInBodyAndPrimaryImage(t *testing.T) {
	source := `<head><meta property="og:image" content="/logo.png"></head><body><script type="application/ld+json">{"@type":"WebPage","mainEntity":{"@type":"Product","name":"Cafetera","image":{"contentUrl":"/coffee.jpg"}}}</script>`
	m := Parse(strings.NewReader(source), "https://shop.example/p")
	if m.Title != "Cafetera" || m.ImageURL != "https://shop.example/coffee.jpg" {
		t.Fatal(m)
	}
	for _, title := range []string{"Mercado Libre", "Access denied", "Just a moment...", ""} {
		if usefulTitle(title, "https://www.mercadolibre.com.pe/p") {
			t.Fatal(title)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestStorefrontAPIsMatchExactProductAndValidateImage(t *testing.T) {
	cases := []struct{ page, marker, path, body, title, image string }{
		{"https://shop.example/products/cup", "", "/products/cup.js", `{"title":"Cup","featured_image":"//cdn.example/cup.jpg"}`, "Cup", "https://cdn.example/cup.jpg"},
		{"https://shop.example/cup/p", "vtex", "/api/catalog_system/pub/products/search/cup/p", `[{"linkText":"other","productName":"Wrong"},{"linkText":"cup","productName":"Cup","items":[{"images":[{"imageUrl":"https://127.0.0.1/private"}]}]}]`, "Cup", ""},
		{"https://shop.example/cup/p", "vtex", "/api/catalog_system/pub/products/search/cup/p", `[{"linkText":"other","productName":"Wrong"}]`, "", ""},
	}
	for _, c := range cases {
		client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != c.path || r.URL.Host != "shop.example" {
				t.Fatal(r.URL)
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(c.body))}, nil
		})}
		u, _ := url.Parse(c.page)
		m := storefront(context.Background(), client, u, []byte(c.marker))
		if m.Title != c.title || m.ImageURL != c.image {
			t.Fatal(m)
		}
	}
}

func TestProductMicrodataAndBodyMetadataIsolation(t *testing.T) {
	source := `<head><title>Store</title></head><body><div itemscope itemtype="https://schema.org/Product"><span itemprop="name">Tazas <b>de cerámica</b></span><img itemprop="image" src="/cup.jpg"><div itemprop="offers" itemscope itemtype="https://schema.org/Offer"><span itemprop="name">Seller name</span></div></div><meta property="og:title" content="Wrong"></body>`
	m := Parse(strings.NewReader(source), "https://shop.example/cup")
	if m.Title != "Tazas de cerámica" || m.ImageURL != "https://shop.example/cup.jpg" {
		t.Fatal(m)
	}
}

func TestMercadoLibreUsesPublicShareMetadataProfile(t *testing.T) {
	req := previewRequest(context.Background(), "https://www.mercadolibre.com.pe/up/MPEU3667914835?pdp_filters=item_id:MPE913032834")
	if !strings.Contains(req.UserAgent(), "WhatsApp/") || !strings.Contains(req.UserAgent(), "TheDate/") || req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
		t.Fatal(req.Header)
	}
	req = previewRequest(context.Background(), "https://mercadolibre.com.pe.evil.test/product")
	if strings.Contains(req.UserAgent(), "WhatsApp/") {
		t.Fatal("profile leaked to unrelated host")
	}
	source := `<title>Microondas Freestanding Eléctrico Electrolux 25l Emdy25s2mxb</title><meta property="og:title" content="Microondas Freestanding Eléctrico Electrolux 25l Emdy25s2mxb"/><meta property="og:image" content="https://http2.mlstatic.com/D_NQ_NP_879774-MPE101077547301_122025-OO.png"/>`
	data := Parse(strings.NewReader(source), "https://www.mercadolibre.com.pe/up/MPEU3667914835")
	if !strings.Contains(data.Title, "Electrolux 25l") || data.ImageURL != "https://http2.mlstatic.com/D_NQ_NP_879774-MPE101077547301_122025-OO.png" {
		t.Fatal(data)
	}
}
