package httpapi

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	email "github.com/lucast1574/thedate.now-back/internal/mail"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func resendConfig(product string) email.Config {
	prefix, sender := "THE_DATE", "The Date <hola@thedate.now>"
	if product == "wedding" {
		prefix, sender = "SAVE_THE_DATE", "Save the Date <savethedate@thedate.now>"
	}
	return email.Config{APIKey: env(prefix+"_RESEND_API_KEY", os.Getenv("RESEND_API_KEY")), From: env(prefix+"_MAIL_FROM", sender)}
}

func (s *server) mailConfigured() bool { return resendConfig(s.product).Valid() }

func (s *server) emailWorker(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, job := range [][2]string{{"users", "welcome"}, {"supportTickets", "ticket_received"}, {"supportTickets", "ticket_resolved"}} {
				if err := s.deliverEmail(ctx, job[0], job[1]); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
					log.Printf("Email delivery pending for product %s", s.product)
				}
			}
		}
	}
}

func (s *server) deliverEmail(ctx context.Context, collection, kind string) error {
	config := resendConfig(s.product)
	if !config.Valid() {
		return nil
	}
	repo := mongostore.Emails{DB: s.db}
	lease, now := uuid.NewString(), time.Now().UTC()
	job, err := repo.Claim(ctx, collection, kind, lease, now)
	if err != nil {
		return err
	}
	delivery := job.Mail[kind]
	payload := delivery.Payload
	if payload == nil {
		name, address, action := job.Name, job.Email, studioFor(s.product)+"/"
		if collection == "supportTickets" {
			name, address, action = job.RequesterName, job.RequesterEmail, studioFor(s.product)+"/support"
		}
		content, renderErr := email.Render(s.product, kind, email.TemplateData{Name: name, Title: job.Title, ActionURL: action})
		if renderErr != nil {
			return renderErr
		}
		brand := "The Date"
		if s.product == "wedding" {
			brand = "Save the Date"
		}
		subject := map[string]string{"welcome": "Tu cuenta está lista", "ticket_received": "Recibimos tu ticket", "ticket_resolved": "Tu ticket está resuelto"}[kind]
		payload = &core.EmailPayload{From: config.From, To: address, Subject: brand + " · " + subject, HTML: content.HTML, Text: content.Text}
		if err = repo.Prepare(ctx, collection, kind, job.ID, lease, *payload); err != nil {
			return err
		}
	}
	config.From = payload.From
	result := email.SendResend(ctx, config, payload.To, payload.Subject, payload.HTML, payload.Text, "date:"+s.product+":"+job.ID+":"+kind)
	state, delay := "sent", time.Duration(0)
	if result.State != "sent" {
		state = "pending"
		delay = time.Minute * time.Duration(1<<min(delivery.Attempts, 6))
	}
	return repo.Finish(ctx, collection, kind, job.ID, lease, state, time.Now().UTC().Add(delay))
}
