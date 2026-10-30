package linkpreview

import (
	"bytes"
	"context"
	"errors"
	"golang.org/x/net/html/charset"
	"io"
	"net/http"
	"strings"
	"time"
)

func Fetch(ctx context.Context, raw string) (Metadata, error) {
	if !ValidURL(raw) {
		return Metadata{}, errors.New("Usa un enlace HTTPS público de la tienda.")
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	raw = NormalizeURL(raw)
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
	// Read bounded HTML, including product JSON-LD placed after the head.
	reader, err := charset.NewReader(io.LimitReader(res.Body, 4<<20), res.Header.Get("Content-Type"))
	if err != nil {
		return Metadata{}, errors.New("No pudimos leer la ficha del producto.")
	}
	body, _ := io.ReadAll(io.LimitReader(reader, 4<<20))
	data := Parse(bytes.NewReader(body), res.Request.URL.String())
	if data.ImageURL == "" || !usefulTitle(data.Title, raw) {
		if extra := storefront(ctx, client, res.Request.URL, body); extra.Title != "" {
			data = extra
		}
	}
	data.URL = raw
	if !usefulTitle(data.Title, raw) {
		return Metadata{}, errors.New("La tienda no compartió la ficha de este producto. Conserva el enlace y completa su nombre e imagen.")
	}
	return data, nil
}
