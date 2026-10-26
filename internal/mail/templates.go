package mail

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"strings"
	texttemplate "text/template"
)

//go:embed templates/*.html templates/*.txt
var emailTemplates embed.FS

type TemplateData struct{ Name, Title, ActionURL, Inviter string }
type Content struct{ HTML, Text string }

// Render uses the committed output of emails/*.tsx; runtime values stay escaped.
func Render(product, kind string, data TemplateData) (Content, error) {
	if product != "wedding" {
		product = "general"
	}
	if !strings.HasPrefix(data.ActionURL, "https://") {
		return Content{}, errors.New("email action must use HTTPS")
	}
	path := "templates/" + product + "-" + kind
	htmlSource, err := emailTemplates.ReadFile(path + ".html")
	if err != nil {
		return Content{}, err
	}
	plainSource, err := emailTemplates.ReadFile(path + ".txt")
	if err != nil {
		return Content{}, err
	}
	htmlTemplate, err := template.New(kind).Parse(string(htmlSource))
	if err != nil {
		return Content{}, err
	}
	plainTemplate, err := texttemplate.New(kind).Parse(string(plainSource))
	if err != nil {
		return Content{}, err
	}
	var html, plain bytes.Buffer
	if err = htmlTemplate.Execute(&html, data); err != nil {
		return Content{}, err
	}
	if err = plainTemplate.Execute(&plain, data); err != nil {
		return Content{}, err
	}
	return Content{HTML: html.String(), Text: plain.String()}, nil
}
