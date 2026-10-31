# BID-NOTIFY-002A — identity email delivery composition

Status: **IMPLEMENTED_NOT_VERIFIED / UNMOUNTED**

## Outcome

Email-verification and password-recovery messages can be composed through the exact active immutable Platform SMTP version and the BID-NOTIFY-001 delivery recorder. Existing environment-configured pilot delivery remains unchanged; the new composition is not activated.

## Authoritative source binding

The identity application now supplies the ledger adapter with authoritative post-commit metadata:

- verification challenge ID, user ID, optional actor ID and correlation ID;
- recovery challenge ID, resolved user ID and correlation ID;
- the in-memory single-use token and expiration time.

A legacy SMTP adapter remains supported for the existing runtime. A ledger-backed adapter is detected through the additive evidence interfaces; it rejects calls that omit authoritative metadata.

Logical notification categories and sources are fixed:

| Category | Source | Template |
|---|---|---|
| `EMAIL_VERIFICATION` | `EMAIL_VERIFICATION_CHALLENGE` | `identity.email_verification` v1 |
| `PASSWORD_RECOVERY` | `PASSWORD_RECOVERY_CHALLENGE` | `identity.password_recovery` v1 |

The challenge UUID is the logical-notification idempotency key. A resend creates a new challenge and notification; replay of the same challenge converges on the existing metadata and attempt.

## Exact active SMTP delivery

`platformsettings/persistence.ActiveSMTPStore` reads one server-owned active `SMTP` version together with its internal secret references. It reconstructs and hash-verifies the immutable configuration. `platformsettings/smtpdelivery.Active` resolves its credentials and trust roots only through the provider-neutral callback resolver and constructs the authenticated STARTTLS transport for the callback lifetime.

Before SMTP is called, the notification recorder:

1. HMAC-fingerprints the canonical recipient and exact prepared MIME bytes;
2. stores the logical notification without recipient or body;
3. rechecks that the selected SMTP version is still the exact active version;
4. appends attempt 1 and minimal audit/outbox evidence atomically.

The SMTP call then runs outside SQL. A clean return records `SMTP_ACCEPTED`; every transport error records `AMBIGUOUS/PROVIDER_OUTCOME_UNKNOWN`. Verification delivery returns its safe failure to the registration flow, which retains its generic public response behavior. Recovery deliberately retains its generic response and ignores the delivery error after evidence recording.

## Content and secret handling

`deliverysmtp.Prepared` keeps recipient and rendered MIME bytes in memory, formats as redacted and exposes only a copied byte slice for keyed fingerprinting. The raw token, recipient, body, credentials and trust roots are never inserted into notification, audit or outbox tables.

The active configuration and external secrets must resolve before the ledger-backed SMTP callback can begin. A configuration/secret-resolution failure therefore returns as unavailable without claiming an SMTP attempt. No automatic retry is introduced.

## Composition and exclusions

`pilot.BuildIdentityNotificationDelivery` composes the active SMTP store, external resolver, notification store/service, HMAC recorder and verification/recovery adapter. It deliberately does not replace the current `composeIdentity` delivery or mount any route.

Excluded:

- secret-manager selection/provisioning;
- runtime or route activation;
- tenant/platform invitations and security notifications;
- retries, queues, bounce handling or delivery receipts;
- migration execution or real SMTP calls;
- frontend/history UI;
- functional, PostgreSQL, crash-window and provider acceptance evidence.
