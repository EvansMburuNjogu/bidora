# BID-SMTP-002 — Platform SMTP HTTP composition

Status: **IMPLEMENTED_NOT_VERIFIED / UNMOUNTED**

## Outcome

An authorized Platform Integration Administrator can create and synthetically test an immutable SMTP configuration version, propose activation or rollback, and a distinct effective Platform Owner can approve or reject the exact unexpired change. The source implementation mounts nothing and selects no secret-manager provider.

## HTTP boundary

`RegisterPlatformIntegrationSettingsWriteRoutes` defines the existing reviewed operations:

- `POST /v1/platform/integration-settings/SMTP/versions`
- `POST /v1/platform/integration-settings/SMTP/versions/{version_id}/test`
- `POST /v1/platform/integration-settings/SMTP/versions/{version_id}/activation-proposals`
- `POST /v1/platform/integration-setting-changes/{change_id}/approve`
- `POST /v1/platform/integration-setting-changes/{change_id}/reject`

All mutations require the authenticated Platform context, exact Origin/session CSRF validation through the existing middleware, a UUID idempotency key, strict 4 KiB JSON and recent TOTP-backed AAL2. Versioned operations require an exact quoted positive `If-Match`. Unknown/duplicate/missing fields, query strings, unsupported media types and non-SMTP setting kinds fail closed. Responses are private/no-store and never contain credential or trust-root references.

The application retains the existing capability boundaries:

- create: `platform.integration.smtp.manage`;
- test: `platform.integration.smtp.test`;
- activation/rollback proposal: `platform.integration.smtp.activate` and rollback capability where applicable;
- decision: distinct effective Platform Owner with `platform.governance.approve`.

Every state change commits immutable state, command replay evidence, audit and outbox facts atomically. Activation rechecks the exact applicable passed test, active-version expectation, maker/checker separation and 30-minute proposal lifetime.

## Provider-neutral secret resolution

`internal/platformsettings/smtptest.SecretResolver` exposes only:

```go
WithSecret(context.Context, reference string, use func([]byte) error) error
```

No provider SDK, ambient environment discovery, storage API or fallback is selected. Implementations remain responsible for authenticated secret access, access audit, rotation and clearing temporary bytes after the callback.

The credential reference resolves to strict UTF-8 JSON containing exactly `username` and `password`. Duplicate, unknown, missing or oversized values fail closed. The trust-root reference resolves to a bounded PEM certificate bundle. Neither resolved value is returned, logged or persisted by this composition.

## Synthetic SMTP test

The tester re-resolves the acting administrator's current verified email and active Platform Administrator affiliation after initial authorization. It sends one fixed synthetic message containing no tenant data through explicit authenticated STARTTLS with verified roots and TLS 1.2 or newer. Expected resolver/provider failures collapse to allowlisted result codes only:

- `SMTP_STARTTLS_VERIFIED`
- `CONNECTIVITY_FAILED`
- `AUTHENTICATION_FAILED`
- `TLS_FAILED`
- `PROVIDER_REJECTED`
- `INVALID_RESPONSE`

The provider call runs outside SQL. The application then rechecks current capability, exact configuration hash/version and recent TOTP AAL2 before committing the immutable redacted test result. A passed test proves only that the SMTP server accepted this synthetic message during that attempt; it does not prove inbox delivery.

The current standard-library SMTP adapter applies the configured delivery deadline and a conservative transport deadline equal to the shortest configured connect, command or delivery timeout. Per-command timeout precision remains a verification/hardening item.

## Composition and exclusions

`pilot.BuildPlatformSMTPWriteHandler` composes the control store, verified-recipient directory, provider-neutral resolver, synthetic tester, write service, redacted reader and strict HTTP handler. It deliberately does not register routes.

Excluded:

- secret-manager selection or provisioning;
- raw secret storage in PostgreSQL;
- route activation or frontend;
- OpenAI setting writes;
- migration execution;
- real SMTP/provider calls;
- notification-producer/ledger wiring;
- retry scheduling, bounce handling or mailbox-delivery claims.
