# The Date API

Go API with MongoDB, Stripe test checkout, MinIO storage and Wazend integration. Use the variable names in `.env.example`. Secrets must be supplied in Dokploy environment settings, never committed. Stripe checkout is available only with a test secret key and a test webhook signing secret. Each successful payment covers one event: USD 25 for a wedding, USD 5 for a general event.

Planner and organizer accounts are free and receive one private demo invitation in their studio. Demos support editing and image uploads, but cannot be published, charged or used to send invitations. An admin verified through Google with `ADMIN_EMAIL` can manage both event types and publish without checkout. A paid wedding planner may create up to two couple accounts; those accounts can edit only the invitation design and its images.

The API uses two private S3 buckets: `thedate-weddings` and `thedate-events`. It creates them on startup when S3 credentials are configured.

Published invitation hosts are computed from the event type and DNS-safe slug:

- Wedding: `<slug>.save.thedate.now`
- General event: `<slug>.thedate.now`

`maybe` responses reserve seats for the configured number of hours. A reason is required for website RSVP; Wazend's native Maybe response sends a follow-up link to collect it. Reservations and capacity changes now use atomic event-document writes with optimistic concurrency, including across API replicas. Read the migration requirements in `docs/rollout.md` before deploying.

## Local development

Use Go 1.27 or later, MongoDB and environment values based on `.env.example`. Run `go test ./...` and `go run ./cmd/api`.

## Webhooks

- Stripe test endpoint: `POST /webhooks/stripe`. Configure its signing secret for this endpoint. Prices are created inline by Checkout in test mode.
- Wazend endpoint: `POST /webhooks/wazend`. Configure `event.response` delivery with an HMAC key matching `WAZEND_WEBHOOK_HMAC`.

## Code navigation and architecture

Start with `docs/backend-map.md`: pure business rules in `internal/core`, orchestration in `internal/application`, typed MongoDB repositories in `internal/mongostore`, centralized Dokploy integration in `internal/dokploy`, and HTTP adapters in `internal/httpapi`. `cmd/api` only starts the process.

Each published invitation gets its own Dokploy project/application using the dedicated The Date organization key and an immutable frontend image. Publication is asynchronous, persists checkpoints, and verifies the container before publishing. Configure the new variables in `.env.example`; operational requirements and remaining limits are described in `docs/rollout.md`.

Authentication revokes legacy sessions on rollout and removes unverified local credentials when an authoritative Google identity claims the same email. Public invitation responses expose only an explicit field allowlist. Shared account rate limits, strict design validation, bounded photo uploads, HTTP timeouts and graceful shutdown are enabled.

For integration tests, provide `TEST_MONGODB_URI` pointing at an isolated local MongoDB. Tests create and drop randomized `thedate_test_*` databases; they never use `MONGODB_DATABASE`.
