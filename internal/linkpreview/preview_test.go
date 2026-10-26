package linkpreview

import (
	"net/netip"
	"strings"
	"testing"
)

func TestLinksNeverTargetPrivateNetworks(t *testing.T) {
	for _, raw := range []string{"http://example.com/item", "https://localhost/item", "https://127.0.0.1/item", "https://169.254.169.254/latest/meta-data", "https://[::1]/", "https://[::ffff:192.168.1.1]/", "https://user:pass@example.com/item", "https://example.com:443/item", "https://name.internal/item"} {
		if ValidURL(raw) {
			t.Fatal("unsafe URL", raw)
		}
	}
	for _, ip := range []string{"10.0.0.1", "100.64.0.1", "172.16.1.1", "192.168.1.1", "192.0.2.1", "198.18.0.1", "224.0.0.1", "240.0.0.1", "fc00::1", "fe80::1", "2001:db8::1"} {
		if PublicIP(netip.MustParseAddr(ip)) {
			t.Fatal("unsafe IP", ip)
		}
	}
	if !ValidURL("https://www.falabella.com.pe/producto") || !PublicIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("public URL rejected")
	}
}
func TestMetadataParsesOpenGraphWithSafeRelativeImages(t *testing.T) {
	source := `<html><head><title>Fallback</title><meta content="Cafetera &amp; tazas" property="og:title"><meta property="og:image" content="/images/coffee.png"><meta name="description" content="Una idea especial"></head><body><meta property="og:title" content="Untrusted body"></body></html>`
	data := Parse(strings.NewReader(source), "https://shop.example/item")
	if data.Title != "Cafetera & tazas" || data.ImageURL != "https://shop.example/images/coffee.png" || data.Description != "Una idea especial" {
		t.Fatal(data)
	}
	data = Parse(strings.NewReader(`<meta property="og:image" content="https://127.0.0.1/a.png">`), "https://shop.example/item")
	if data.ImageURL != "" {
		t.Fatal("private image accepted")
	}
}
