# The Date API

Go API with MongoDB, Stripe test checkout, MinIO storage and Wazend integration. Use the variable names in `.env.example`. Secrets must be supplied in Dokploy environment settings, never committed. Stripe checkout is available only with a test secret key and a test webhook signing secret. Each successful payment covers one event: USD 25 for a wedding, USD 5 for a general event.

Planner and organizer accounts are free and receive one private demo invitation in their studio. Demos support editing and image uploads, but cannot be published, charged or used to send invitations. An admin verified through Google with `ADMIN_EMAIL` can manage both event types and publish without checkout. A paid wedding planner may create up to two couple accounts; those accounts can edit only the invitation design and its images.

The API uses two private S3 buckets: `thedate-weddings` and `thedate-events`. It creates them on startup when S3 credentials are configured.

Published invitation hosts are computed from the event type and DNS-safe slug:

- Wedding: `<slug>.save.thedate.now`
- General event: `<slug>.thedate.now`

`maybe` responses reserve seats for the configured number of hours. A reason is required for website RSVP; Wazend's native Maybe response sends a follow-up link to collect it. The application should run as one API replica until a database-level capacity reservation mechanism is added.

## Local development

Use Go 1.27 or later, MongoDB and environment values based on `.env.example`. Run `go test ./...` and `go run ./cmd/api`.

## Webhooks

- Stripe test endpoint: `POST /webhooks/stripe`. Configure its signing secret for this endpoint. Prices are created inline by Checkout in test mode.
- Wazend endpoint: `POST /webhooks/wazend`. Configure `event.response` delivery with an HMAC key matching `WAZEND_WEBHOOK_HMAC`.
