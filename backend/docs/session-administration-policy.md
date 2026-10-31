# Pilot browser-session administration policy preparation

Status: **APPROVED — IMPLEMENTED_NOT_VERIFIED**  
Prepared: 2026-10-18  
Scope: a signed-in user revoking one other browser session or revoking every active browser session for their own account.

## Why this policy is needed

Users can currently inspect active sessions and log out the current browser, but they cannot remove another active session or respond to suspected account compromise by invalidating all sessions. These operations must remain account-owner-only, avoid device-tracking data, serialize against session issuance and produce minimal security evidence.

## Recommended policy

### Common authority and browser controls

- Only the authenticated user may administer that same user's sessions. Tenant membership, role labels, platform-admin status and tenant headers confer no session authority.
- Both operations require the current valid session, exact approved HTTPS Origin, matching session-bound CSRF header/cookie values, and a session created within the previous 15 minutes.
- Requests have no body or query parameters. The session identifier is an opaque UUID, never a cookie/token/hash.
- Validation occurs before idle touch. Failures never extend session lifetime.
- Audit/outbox evidence contains only user ID, action, target session ID or resulting auth epoch, actor session ID, timestamp and correlation ID. It contains no token/CSRF hash, email, IP address, user agent, inferred device/location or tenant-private content.

### Revoke one other session

- `DELETE /v1/auth/sessions/{session_id}` revokes only an active, unexpired session belonging to the current user and current auth epoch.
- The target must differ from the current session. Current-session termination continues to use the existing logout route so cookie clearing remains explicit.
- The command locks the target session, sets `revoked_at` once, and writes audit/outbox evidence in the same control transaction.
- A required UUID `Idempotency-Key` identifies the command. Only its SHA-256 hash and a versioned request hash are persisted in bounded session-revocation command evidence; matching replay returns `204` and conflicting reuse fails safely.
- A target that is absent, belongs to another user, expired or already revoked returns the same generic `204` without creating false success evidence. Targeting the current session returns `409 CURRENT_SESSION_REQUIRES_LOGOUT` because the user already knows that identifier from the private inventory.
- The current session and its CSRF value remain unchanged.

### Revoke all sessions

- `POST /v1/auth/sessions/revoke-all` means **all active sessions including the current session**. “All other” is not silently substituted.
- The command locks the user and current session, verifies the current auth epoch, fails closed if more than 100 active current-epoch sessions exist, advances `identity.users.auth_epoch` exactly once, revokes all active current-epoch sessions and writes one aggregate audit/outbox fact atomically.
- Advancing auth epoch makes an old-epoch session fail validation even if a concurrent issuance attempt inserts after the row update. A login that begins after the committed epoch advance may create a new valid session.
- The successful response is generic `204` and clears both approved session and CSRF cookies. No replacement session is issued.
- This one-shot operation is naturally retry-safe but cannot provide authenticated replay after success because the calling session is invalid. It therefore does not require an idempotency key and writes evidence only for the transaction that advances the epoch.
- If the transaction outcome is ambiguous to the browser, the user must authenticate again; successful login proves the previous session can no longer authorize requests.

### Inventory behavior

After approval and implementation, `GET /v1/auth/sessions` will report:

- `revocation_policy_configured: true`;
- `other_session_revocation_available: true` only when another active session exists;
- `revoke_all_available: true` while the current session is valid;
- no `SESSION_ADMINISTRATION_POLICY_NOT_CONFIGURED` blocker.

The inventory remains capped at 100 sessions and continues to expose no device metadata.

## Proposed contracts

- `DELETE /v1/auth/sessions/{session_id}` — revoke one other session; no body/query; UUID idempotency required; `204`, generic concealed `204`, `409` for current session or idempotency conflict, and fixed authentication/browser/dependency failures.
- `POST /v1/auth/sessions/revoke-all` — revoke all including current; no body/query; `204` with both cookies cleared, `409` when the active-session bound is exceeded, and fixed authentication/browser/dependency failures.

## Implementation boundary if approved

- Add one bounded control migration for hash-only revoke-one idempotency evidence; existing `identity.sessions.revoked_at`, `identity.users.auth_epoch`, control audit and control outbox support the state transitions.
- Add locked sqlc commands, application services, pgx transaction committers, strict Fiber handlers, pilot composition and updated session inventory semantics.
- Preserve the existing current-session logout behavior.
- Deferred verification must cover concurrent target replay, current-session targeting, cross-user concealment, revoke-all versus concurrent login/issuance, auth-epoch invalidation, audit/outbox rollback, cookie clearing and the 100-session bound.

## Explicit exclusions

No administrator-initiated user-session revocation, device naming, trusted-device state, IP/user-agent collection, geolocation, notification delivery, session restoration, MFA enrollment, step-up token, refresh token or tenant-scoped session is included.

## Approval evidence

The requesting user approved BID-ADR-022 on 2026-10-18 with account-owner-only authority, 15-minute session freshness, concealed hash-idempotent other-session revocation, explicit current-session rejection, revoke-all meaning all active sessions including the caller, auth-epoch advancement, the 100-session bound, cookie clearing without replacement and the listed privacy exclusions. Source implementation remains `IMPLEMENTED_NOT_VERIFIED`; migration execution, concurrency verification, security review and UAT are deferred.
