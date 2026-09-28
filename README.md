# The Date API

Go API with MongoDB, Stripe test checkout, MinIO storage and Wazend integration. Use the variable names in `.env.example`. Secrets must be supplied in Dokploy environment settings, never committed.

The API uses two private S3 buckets: `thedate-weddings` and `thedate-events`. It creates them on startup when S3 credentials are configured.

Published invitation hosts are computed from the event type and DNS-safe slug:

- Wedding: `<slug>.save.thedate.now`
- General event: `<slug>.thedate.now`

`maybe` responses reserve seats for the configured number of hours. A reason is required for website RSVP; Wazend's native Maybe response sends a follow-up link to collect it. The application should run as one API replica until a database-level capacity reservation mechanism is added.

## Local development

Use Go 1.27 or later, MongoDB and environment values based on `.env.example`. Run `go test ./...` and `go run ./cmd/api`.

## Webhooks

- Stripe test endpoint: `POST /webhooks/stripe`. Configure its signing secret and use Stripe test price IDs.
- Wazend endpoint: `POST /webhooks/wazend`. Configure `event.response` delivery with an HMAC key matching `WAZEND_WEBHOOK_HMAC`.
