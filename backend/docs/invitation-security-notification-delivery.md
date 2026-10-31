# BID-NOTIFY-002B — invitation and Platform security delivery

Status: **BID-NOTIFY-002B and BID-NOTIFY-002C IMPLEMENTED_NOT_VERIFIED / UNMOUNTED**

## Outcome

Tenant invitation, Platform Administrator invitation and the existing Platform security email templates now have ledger-backed exact-active-SMTP delivery adapters. Current runtime composition remains unchanged.

## Tenant invitations

The Tenderpreneur invitation application supplies an additive evidence request after the invitation transaction commits. It contains the authoritative invitation, actor, tenant and correlation IDs, fixed role, recipient, single-use token and expiry. Legacy delivery remains supported; a ledger-backed adapter rejects metadata-free calls.

The adapter records:

- category `TENANT_INVITATION`;
- source `TENANT_INVITATION` and exact invitation ID;
- template `tenant.invitation` v1;
- tenant and actor provenance;
- HMAC recipient and exact prepared-MIME fingerprints.

The invitation UUID is the logical notification idempotency key.

## Platform invitations

Direct and maker/checker-approved Platform invitations pass the authoritative invitation ID, initiating actor/checker, correlation ID, role, recipient, token and expiry to the evidence-aware adapter. The adapter renders the existing safe Platform invitation template and records category/source `PLATFORM_INVITATION` with template `platform.invitation.issued` v1. Pending invitations produce no email until approved and activated.

## Platform security templates

The adapter also exposes metadata-required delivery operations for the existing templates:

| Category/source | Template |
|---|---|
| `PLATFORM_SESSION_REVOCATION` | `platform.session_revocation` v1 |
| `PLATFORM_IDENTITY_STATE_CHANGE` | `platform.identity_state` v1 |
| `PLATFORM_ROLE_CHANGE` | `platform.role_change` v1 |

Each operation requires the authoritative change/session source ID, actor, recipient user, correlation ID, recipient address and occurrence time.

BID-NOTIFY-002C adds one optional application delivery port. `NewModuleWithAdministration` enables it only when the selected invitation adapter also implements the security port. Direct role and identity changes, approved maker/checker owner changes, and explicit administrator session revocation invoke it only after the authoritative transaction commits. Pending and rejected changes, failed transactions and idempotent replays do not send.

Delivery errors are deliberately ignored after commit. SMTP, secret-resolution, notification-ledger or request-cancellation failure therefore cannot roll back, weaken or falsely report failure of the completed role, identity or session security operation.

## Shared delivery behavior

All adapters:

1. load and hash-verify the exact active immutable SMTP configuration;
2. resolve external credentials and trust roots through the provider-neutral callback resolver;
3. prepare exact MIME bytes in memory;
4. use BID-NOTIFY-001 to record the logical notification and attempt 1 before SMTP;
5. recheck the exact SMTP version remains active;
6. send outside SQL;
7. record `SMTP_ACCEPTED` or fail-closed `AMBIGUOUS/PROVIDER_OUTCOME_UNKNOWN`.

No address, body, token, credential, secret reference or provider diagnostic is persisted in notification/audit/outbox evidence.

## Composition and exclusions

`pilot.BuildInvitationNotificationDelivery` composes and returns the tenant and Platform adapters without replacing current runtime delivery or registering routes.

Excluded:

- runtime/route activation;
- secret-manager selection or provisioning;
- invitation reminders or reminder scheduling;
- retries, bounce handling or mailbox-delivery claims;
- migration execution, real SMTP calls, frontend/history UI or provider acceptance.
