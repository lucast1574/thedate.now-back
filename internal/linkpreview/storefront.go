package linkpreview

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Public storefront APIs complement HTML metadata; no scripts are executed,
// no browser cookies are forwarded and every destination uses the checked client.
func storefront(ctx context.Context, client *http.Client, page *url.URL, body []byte) Metadata {
	parts := strings.Split(strings.Trim(page.Path, "/"), "/")
	endpoint := *page
	endpoint.RawQuery, endpoint.Fragment = "", ""
	if len(parts) == 2 && parts[0] == "products" {
		endpoint.Path = page.Path + ".js"
		var product struct {
			Title         string `json:"title"`
			Description   string `json:"description"`
			FeaturedImage string `json:"featured_image"`
		}
		if readStoreJSON(ctx, client, endpoint.String(), &product) {
			return checkedStoreMetadata(Metadata{Title: product.Title, ImageURL: product.FeaturedImage, Description: product.Description}, page)
		}
	}
	if len(parts) == 2 && parts[1] == "p" && strings.Contains(strings.ToLower(string(body)), "vtex") {
		endpoint.Path = "/api/catalog_system/pub/products/search/" + parts[0] + "/p"
		var products []struct {
			Name        string `json:"productName"`
			LinkText    string `json:"linkText"`
			Description string `json:"description"`
			Items       []struct {
				Images []struct {
					URL string `json:"imageUrl"`
				} `json:"images"`
			} `json:"items"`
		}
		if readStoreJSON(ctx, client, endpoint.String(), &products) {
			for _, product := range products {
				if product.LinkText != parts[0] {
					continue
				}
				image := ""
				if len(product.Items) > 0 && len(product.Items[0].Images) > 0 {
					image = product.Items[0].Images[0].URL
				}
				return checkedStoreMetadata(Metadata{Title: product.Name, ImageURL: image, Description: product.Description}, page)
			}
		}
	}
	return Metadata{}
}
func readStoreJSON(ctx context.Context, client *http.Client, raw string, dst any) bool {
	if !ValidURL(raw) {
		return false
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	req.Header.Set("User-Agent", "TheDate/1.0 (+https://thedate.now; product preview)")
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if (res.StatusCode != 200 && res.StatusCode != 206) || !strings.Contains(res.Header.Get("Content-Type"), "json") {
		return false
	}
	return json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(dst) == nil
}
func checkedStoreMetadata(data Metadata, page *url.URL) Metadata {
	if !usefulTitle(data.Title, page.String()) {
		return Metadata{}
	}
	data.Title = truncate(data.Title, 240)
	data.Description = truncate(data.Description, 1000)
	if image, err := page.Parse(data.ImageURL); err == nil && data.ImageURL != "" && ValidURL(image.String()) {
		data.ImageURL = image.String()
	} else {
		data.ImageURL = ""
	}
	data.URL = page.String()
	return data
}
