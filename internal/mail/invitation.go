package mail

import (
	"bytes"
	"html/template"
)

type Invitation struct{ Brand, Inviter, Title, URL, Color, Background, Heading string }

func AccessInvitation(kind, inviter, title, url string) string {
	data := Invitation{"The Date", inviter, title, url, "#7146ba", "#f4f0ff", "Un gran evento empieza en equipo."}
	if kind == "wedding" {
		data.Brand = "Save the Date"
		data.Color = "#996b50"
		data.Background = "#faf5ed"
		data.Heading = "Una historia que pueden preparar juntos."
	}
	t := template.Must(template.New("invite").Parse(accessHTML))
	var b bytes.Buffer
	_ = t.Execute(&b, data)
	return b.String()
}

const accessHTML = `<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body style="margin:0;background:{{.Background}};color:#302a32;font-family:Arial,sans-serif"><table role="presentation" width="100%" cellpadding="24"><tr><td align="center"><table role="presentation" width="100%" style="max-width:580px;background:#fff;border-radius:18px" cellpadding="32"><tr><td><p style="letter-spacing:3px;font-size:12px;color:{{.Color}}">{{.Brand}}</p><h1 style="font-family:Georgia,serif;font-size:32px;line-height:1.2">{{.Heading}}</h1><p style="line-height:1.7">{{.Inviter}} te invita a colaborar en <strong>{{.Title}}</strong>.</p><p style="line-height:1.7">Podrás editar la invitación y organizar invitados y mesas de este evento. Tu cuenta será tuya: podrás crear otras bodas y eventos independientes.</p><p style="margin:32px 0"><a href="{{.URL}}" style="display:inline-block;background:{{.Color}};color:#fff;padding:16px 24px;border-radius:8px;text-decoration:none;font-weight:bold">Aceptar acceso al evento</a></p><p style="font-size:13px;line-height:1.7;color:#746d77">Regístrate o inicia sesión con el correo que recibió esta invitación. El enlace vence en 7 días. Este acceso no permite ver otros eventos ni los pagos del organizador.</p><hr style="border:0;border-top:1px solid #eee"><p style="font-size:12px;color:#746d77">Una cuenta. Tus celebraciones. Si no esperabas este correo, puedes ignorarlo.</p></td></tr></table></td></tr></table></body></html>`
