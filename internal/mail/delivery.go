package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type Config struct {
	Host, Port, User, Password, From string
	Secure                           bool
}

// Result distinguishes a rejected attempt from an unknown acknowledgement.
type Result struct{ State string }

func (c Config) Valid() bool {
	a, e := mail.ParseAddress(c.From)
	return e == nil && a.Address != "" && c.Host != "" && c.Port != "" && c.User != "" && c.Password != ""
}
func Send(ctx context.Context, c Config, to, subject, html string) Result {
	failed := Result{"failed"}
	if !c.Valid() || strings.ContainsAny(to+subject, "\r\n") {
		return failed
	}
	address, e := mail.ParseAddress(to)
	if e != nil || address.Address != to {
		return failed
	}
	from, _ := mail.ParseAddress(c.From)
	dialer := net.Dialer{Timeout: 15 * time.Second}
	conn, e := dialer.DialContext(ctx, "tcp", net.JoinHostPort(c.Host, c.Port))
	if e != nil {
		return failed
	}
	defer conn.Close()
	deadline := time.Now().Add(25 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetDeadline(deadline)
	tlsConfig := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	if c.Secure {
		t := tls.Client(conn, tlsConfig)
		if t.HandshakeContext(ctx) != nil {
			return failed
		}
		conn = t
	}
	client, e := smtp.NewClient(conn, c.Host)
	if e != nil {
		return failed
	}
	defer client.Close()
	if !c.Secure {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return failed
		}
		if client.StartTLS(tlsConfig) != nil {
			return failed
		}
	}
	if client.Auth(smtp.PlainAuth("", c.User, c.Password, c.Host)) != nil || client.Mail(from.Address) != nil || client.Rcpt(to) != nil {
		return failed
	}
	writer, e := client.Data()
	if e != nil {
		return failed
	}
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", from.String(), to, mime.QEncoding.Encode("UTF-8", subject), html)
	if _, e = writer.Write([]byte(body)); e != nil {
		return Result{"uncertain"}
	}
	if writer.Close() != nil {
		return Result{"uncertain"}
	}
	_ = client.Quit()
	return Result{"sent"}
}
