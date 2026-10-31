# BID-NOTIFY-001 — notification delivery evidence

Status: **IMPLEMENTED_NOT_VERIFIED**

## Approved boundary

The requesting user approved Option A: SMTP configuration uses external secret references only; notification metadata and delivery attempts are append-only; audit/outbox evidence is minimal; message bodies, tokens, credentials and raw provider errors are never persisted.

## Source behavior

Control migration `00030_notification_delivery_evidence.sql` introduces three append-only relations:

- `notification.notifications` records one logical notification, its source, template identity/version, keyed recipient/content fingerprints and replay hash.
- `notification.delivery_attempts` records each started attempt against the exact active immutable SMTP configuration version.
- `notification.delivery_attempt_results` records one safe terminal disposition: `SMTP_ACCEPTED`, `FAILED` or `AMBIGUOUS`.

Every prepare, attempt-start and attempt-finish transaction appends a minimal control audit event and internal outbox fact atomically. Those facts contain notification/attempt IDs, category, template version, attempt number, safe disposition and an allowlisted failure code only. They contain no address, body, template variables, token, credential, secret reference or provider diagnostic.

`SMTP_ACCEPTED` means only that the configured SMTP server accepted the message. It is not mailbox delivery, opening or human receipt evidence.

## Delivery sequencing

`internal/notification/delivery.Recorder`:

1. derives domain-separated HMAC-SHA-256 fingerprints in memory with an explicit versioned key;
2. persists or exactly replays the logical notification;
3. verifies that the selected immutable SMTP version is currently active;
4. appends the next sequential attempt before any SMTP call;
5. performs the supplied SMTP call outside every SQL transaction;
6. appends `SMTP_ACCEPTED` on a clean return or fail-closed `AMBIGUOUS/PROVIDER_OUTCOME_UNKNOWN` on transport error.

A crash after attempt start leaves an immutable started attempt without a result, which is itself evidence of an ambiguous outcome. A later attempt cannot start while an earlier attempt lacks a result or after any attempt is `SMTP_ACCEPTED`. This slice schedules no automatic retry.

## Security and privacy

- Recipient and rendered-content fingerprints require a dedicated deployment-supplied HMAC key of at least 32 bytes; the key is never stored in the database.
- SMTP credentials and trust roots remain external secret-manager references in immutable Platform Administration setting versions.
- New attempts fail closed unless the referenced SMTP version is the exact active `SMTP` setting.
- Runtime grants permit only `SELECT` and `INSERT` on notification evidence; append-only triggers reject update/delete.
- No retention deletion is implemented. The evidence remains retained until an approved retention/legal-hold policy introduces a separately governed disposition operation.

## Exclusions and remaining gates

The source boundary is not mounted into current identity, invitation or Platform Administration delivery flows. Platform SMTP create/test/activate operations remain unmounted pending approved secret resolution and synthetic provider-test composition. This slice adds no notification-history HTTP contract, frontend, worker, automatic retry, bounce/webhook processing, external SMTP call, migration execution or deployment activation.

Before actual notification sending can claim complete ledger coverage, the active SMTP resolver and governed write/test/activation routes must be composed, then every email producer must call the recorder with its authoritative source and template metadata. PostgreSQL transaction, replay, race, crash-window, privacy/log and provider acceptance evidence remains deferred.
