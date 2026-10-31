# Pilot MFA and step-up policy preparation

Status: **APPROVED — IMPLEMENTED_NOT_VERIFIED**  
Approved: 2026-10-18 by requesting user  
Scope: one TOTP factor per user, recovery codes, MFA-gated login, session assurance and step-up for selected Option A operations.

## Why this policy is needed

A recently created password or Google session proves recent primary authentication but not possession of a second factor. Option A now includes package approval, submission confirmation and membership offboarding, so the pilot needs explicit assurance provenance rather than treating session age, role labels or tenant capabilities as MFA.

## Approved policy

### Factor and secret protection

- Pilot MFA uses one RFC 6238 TOTP factor per user: 6 digits, 30-second period, HMAC-SHA-1 for broad authenticator compatibility, and a 20-byte cryptographically random secret.
- The control database stores only AES-256-GCM ciphertext, nonce and key version. AAD binds ciphertext to the user, factor ID and algorithm. The raw secret is returned once during pending enrollment and is never logged, audited, placed in HTTP request URLs or persisted in plaintext.
- The application requires an explicit versioned 32-byte encryption key ring supplied through deployment secret references. There is no fallback/default key. Decryption accepts configured historical versions for rotation; new writes use exactly one configured current version. Production KMS and key-custody acceptance remain deployment gates.
- One pending enrollment and one active factor per user are allowed. Pending enrollment expires after 10 minutes and cannot authorize MFA.

### Enrollment and activation

1. `POST /v1/auth/mfa/totp/enroll` requires the current authenticated session, exact approved HTTPS Origin, session-bound CSRF and a session created within 15 minutes. It invalidates any prior pending enrollment and returns the TOTP secret and canonical `otpauth://` fields once in a private/no-store body.
2. `POST /v1/auth/mfa/totp/confirm` requires the pending enrollment and one valid TOTP code. Five failed confirmations within 10 minutes invalidate the pending enrollment; limiter dependency failure denies the operation.
3. Confirmation activates the factor, generates ten recovery codes, advances auth epoch, revokes all existing sessions and creates one replacement `AAL2` session atomically with factor/recovery/session audit and outbox evidence. Only after commit may the ten raw recovery codes and replacement cookies be returned.

The pilot has no factor-disable or identity-proofing reset operation. Replacing a factor requires a current `AAL2` session and confirmation of the new factor; it preserves immutable history, advances auth epoch and regenerates recovery codes.

### Recovery codes

- Generate ten independent 16-character Crockford Base32 codes from 10 random bytes each; display once.
- Store only domain-separated SHA-256 hashes with factor/user binding. Codes are single-use and consumed atomically with challenge completion.
- Regeneration requires `AAL2` assurance obtained within 15 minutes, invalidates all remaining prior codes and returns a new set once.
- Password recovery never removes/bypasses MFA. If no factor or recovery code is available, the pilot provides no self-service bypass; support reset remains blocked pending a separately approved identity-proofing policy.

### Login challenge

- If no active factor exists, successful password or Google primary authentication continues to issue an `AAL1` session.
- If an active factor exists, successful primary authentication issues no browser session. It creates a 32-byte opaque, five-minute, single-use `MFA_LOGIN` transaction stored only as a hash and bound to user, primary method, auth epoch and browser correlation.
- `POST /v1/auth/mfa/challenge` accepts exactly one TOTP or recovery-code response. TOTP accepts the current 30-second step plus one adjacent step for clock skew, but its counter must be greater than the factor's last accepted counter, preventing replay. Recovery code consumption is atomic.
- Five failed challenge attempts within 10 minutes consume the transaction and deny further use. Redis-backed attempt enforcement is fail closed and uses domain-separated HMAC scope keys; it stores no raw code or email.
- Successful challenge consumes the transaction and issues one `AAL2` session with method and assurance time recorded atomically. Generic failures do not reveal factor or account state.

### Session assurance and step-up

- Sessions store `assurance_level` (`AAL1` or `AAL2`), `assured_at`, and an internal method class (`PRIMARY_ONLY`, `TOTP`, or `RECOVERY_CODE`). These are authorization inputs, not capabilities.
- `POST /v1/auth/mfa/step-up` requires a current session plus Origin/CSRF and verifies TOTP or one recovery code. Success revokes the current session and atomically issues a replacement `AAL2` session linked through rotation; auth epoch is unchanged.
- A step-up is valid for a governed action only when the session is `AAL2`, `assured_at` is no more than 15 minutes old, the session/auth epoch remains current, and authorization is rechecked for that action.
- Recovery-code assurance is accepted for account recovery and factor replacement, but not for proposal approval or submission confirmation. Those operations require recent TOTP assurance.

### Option A enforcement

The following existing operations must require recent TOTP `AAL2` in addition to all current capability, version, separation-of-duty and readiness rules:

- exact-package proposal approval;
- human confirmation of external submission; and
- membership offboarding begin, retarget and cancellation.

MFA does not grant tenant membership, capabilities, ownership or approval authority. Users who need these operations must enroll before performing them. Session revoke-all remains available with its already approved fresh-primary-session rule so a user can respond to suspected compromise without an MFA bypass/reset.

Platform-administrator MFA remains mandatory when platform administration is implemented, but no platform-admin route is added by this pilot increment.

### Privacy and evidence

- Never log or return stored secret ciphertext, raw TOTP/recovery codes, challenge hashes, encryption keys or browser-correlation values.
- Audit/outbox facts contain factor ID, action, assurance method class, policy version, actor/user IDs, timestamps and success outcome only.
- Readiness may report enrollment and assurance status, but never secret material, recovery-code hashes/count values precise enough to expose codes, or provider subjects.

## Proposed contracts

- `POST /v1/auth/mfa/totp/enroll`
- `POST /v1/auth/mfa/totp/confirm`
- `POST /v1/auth/mfa/challenge`
- `POST /v1/auth/mfa/step-up`
- `POST /v1/auth/mfa/recovery-codes/regenerate`
- `POST /v1/auth/mfa/totp/replace`

All use strict 4 KiB JSON, fixed safe errors, private/no-store responses, bounded deadlines and no query parameters. Replacement uses the single approved path in two explicit calls: `{}` begins and returns the pending secret once; `{\"code\":\"......\"}` confirms it. The Redis limiter serializes each hash-scoped challenge, records only failures, denies after five failures for ten minutes and fails closed when unavailable.

## Required implementation if approved

- Control migration for encrypted factor versions, hashed recovery codes, hashed MFA transactions and session assurance columns.
- Standard-library TOTP/HOTP validation and AES-GCM key-ring adapter; no provider SDK or ambient secret discovery.
- Login/Google callback deferral when MFA is active, fail-closed Redis attempt adapter, challenge/enrollment/step-up services, session rotation, contracts and pilot composition.
- Assurance checks added to the three governed Option A operations without weakening their existing authorization and concurrency rules.
- Deferred verification: RFC vectors, clock/replay boundaries, encryption/AAD/key rotation, code concurrency, challenge limits, login deferral, session fixation/rotation, recovery-code races, governed-operation denial and live PostgreSQL/Redis/browser flows.

## Explicit exclusions

No SMS/email OTP, push provider, WebAuthn/passkeys, trusted-device bypass, remembered-browser cookie, administrator factor reset, help-desk bypass, plaintext/exportable stored secret, platform-admin route, risk engine or production KMS selection is included.

## Approval needed

Approval is required for: TOTP parameters; AES-GCM environment-supplied versioned key ring; enrollment/activation session effects; recovery-code count/format; five-minute single-use login transaction; attempt limits; TOTP replay window; stored session assurance; 15-minute step-up lifetime; recovery-code assurance restrictions; the exact three governed operations; no self-service bypass; and the listed exclusions.
