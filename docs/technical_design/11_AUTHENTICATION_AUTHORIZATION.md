# Authentication and authorization

Sources: [SRD](../product/04_SRD.md) SR-IAM-001–008, [security §§5–7](../product/14_SECURITY_PRIVACY_COMPLIANCE.md), RULE-IAM-001–003. Decisions [007, 011, 016, 018](27_ARCHITECTURE_DECISIONS.md). Execution: [identity plan](../implementation/08_AUTHENTICATION_AUTHORIZATION.md).

## Identity proposal and unresolved choices

Go owns membership, capabilities, resource policies and session lifecycle. On 2026-10-18 the requesting user selected Go-owned email/password with Argon2id; managed OIDC is not the Option A implementation path. The backend has a parameter-explicit Argon2id-v19 PHC implementation with no implicit defaults and bounded hostile-PHC parsing. The approved pilot profile is 64 MiB, 3 iterations, parallelism 2, 16-byte salt, 32-byte key and 12–128 valid UTF-8 password bytes without trimming/normalization/composition rules. The confirmed pilot email value accepts strict ASCII dot-atom mailboxes, preserves display casing, lowercases the full lookup identity and performs no provider alias rewriting. Email ownership verification now uses approved 32-byte opaque tokens, SHA-256 at rest, 30-minute expiry, one active `EMAIL_VERIFY` challenge, resend invalidation and atomic single-use consumption; invalid/expired/consumed outcomes remain generic and metadata has no automatic cleanup pending retention policy. Consumption and minimal `EmailVerified` audit evidence commit in one control transaction. The route-independent credential verifier requires attempt-control ports, validates its dummy PHC against the current profile, performs dummy work for malformed/unknown identities, checks verified/non-disabled/epoch state and returns generic failures. The approved session profile uses domain-separated hashes for 32-byte session/CSRF values, `__Host-bidora_session` with Secure/HttpOnly/SameSite=Lax/Path=/ and no Domain, 30-minute idle/12-hour absolute limits, rotation/revocation, memory-only bootstrap CSRF and validated Origin plus `X-CSRF-Token` on unsafe methods. Standard-library browser primitives enforce exact configured HTTPS origins, generic Origin/CSRF failures, unsafe-method checks and host-only set/clear cookie attributes; the pilot Fiber composition uses them for the implemented authentication routes. The approved attempt profile is identity 5 failures/15 minutes then 15-minute deny, and network 30/minute plus 100/15 minutes then 15-minute deny. HMAC-SHA-256 keys conceal raw email/IP; unknown/malformed attempts count; success clears identity failures only; limiter failure denies authentication. The application requires atomic reservation/finish ports to avoid concurrent check/record bypass. Login orchestration then performs any required password rehash and requires session plus security-audit commit before session/CSRF secrets may leave the application boundary. Concrete pgx/sqlc adapters hash outside transactions, use expected credential versions and atomically insert the session plus minimal `UserLoggedIn` audit. A no-default client-IP resolver ignores forwarding headers from untrusted peers and strictly walks explicitly trusted `X-Forwarded-For` chains right-to-left; configured CIDRs must contain proxies only. The Redis controller now maps reservation/finish to atomic Lua scripts with HMAC-only keys, explicit Cluster hash slot and verified-TLS/auth/bounded-client configuration; no server or live script race has been tested. Session-cookie validation rechecks current user/auth epoch in a read-only transaction; idle refresh is a separate post-policy write so rejected cross-site traffic cannot preserve a session. The approved browser binding uses Secure host-only SameSite=Lax `__Host-bidora_session` (HttpOnly) and `__Host-bidora_csrf` (script-readable), with exact HTTPS Origin plus header/cookie/session-hash matching on unsafe methods. The pilot Fiber boundary performs these checks before idle touch and places only a redacted validated principal in request-local state. The approved pilot login handler requires exact Origin, derives the attempt network scope through the trusted-proxy resolver, returns generic failures, and sets both cookies only after session/audit commit; it does not revoke existing sessions. Current-session logout revokes by session-and-user identity and appends `UserLoggedOut` audit evidence atomically; its unregistered handler clears both cookies only after success. Deployment performance/concurrency acceptance, Redis target/secret and actual trusted-proxy values, delivery-provider/recovery policy and generic recovery behavior remain pending and must be approved before production authentication acceptance. Public email verification is composed as strict 4 KiB `{token}` JSON under an exact approved Origin; it creates no session or authority, and invalid/expired/replayed tokens remain generic. For external capture, authorization now requires current verified/non-disabled identity, ACTIVE membership and Tenderpreneur tenant, matching context generation and explicit `tenderpreneur.opportunity.capture_external` ALLOW with active DENY precedence; role labels and platform-admin status never suffice. MFA is mandatory for platform administrators; high-risk tenant assurance policy requires approval. Factor secrets are encrypted under restricted key access; recovery codes are one-way hashed and single-use. BID-ADR-023 now prepares the exact pilot choice: one AES-GCM-encrypted RFC 6238 TOTP factor, ten high-entropy hash-only recovery codes, MFA-gated login, explicit session `AAL1`/`AAL2` provenance and recent TOTP step-up for package approval, submission confirmation and membership offboarding. It remains pending approval and does not yet authorize factor storage or enforcement.

```mermaid
sequenceDiagram
  actor U as User
  participant W as Browser
  participant A as Go identity API
  participant C as Control DB
  U->>W: Authenticate and complete MFA when required
  W->>A: Login/challenge response over TLS
  A->>C: Verify identity and consume challenge atomically
  A->>C: Store hash of new opaque session token
  A-->>W: Secure HttpOnly cookie; no bearer token in localStorage
  W->>A: GET /v1/me
  A->>C: Validate session, epoch and memberships
  A-->>W: Authorized bootstrap and required actions
```

## Browser/session rules

Prefer host-only Secure HttpOnly cookie, appropriate SameSite and bounded absolute/idle lifetime. Exact values are reviewed configuration, not undocumented defaults. Rotate after login, MFA, recovery and privilege elevation. CSRF token/origin checks protect cookie writes; SameSite alone is insufficient. Trusted proxy configuration must not allow attacker-supplied origin/forwarding headers to bypass checks. Private SSR fetches never use shared Next.js caches.

## Capability model

Authorization inputs: actor, active membership, tenant type/status, capability grant/deny, resource assignment, classification, workflow state, conflict/separation of duty and session assurance. Explicit denies and narrower scope win. UI action hints are advisory.

| Operation | Proposed capability | Additional guard |
|---|---|---|
| Create external bid | tenderpreneur.bid.manage | Active workspace, quota, verified source |
| Edit technical response | tenderpreneur.response.edit | Bid/team scope, editable state |
| Read/edit financial response | tenderpreneur.pricing.view / tenderpreneur.pricing.edit | Independent classification permission |
| Approve candidate | tenderpreneur.bid.approve | Assigned policy step, current version, separation-of-duty |
| Generate/download package | tenderpreneur.package.generate / download | Permission for every included classification |
| Confirm external evidence | tenderpreneur.submission.verify | Approved verifier policy, exact package/proof |
| Change roles | tenant.memberships.manage | Grantable role boundary, last-admin protection |
| View private tenant content as support | No standing grant | Approved scoped expiring support session |

Capability names beyond source examples are proposed vocabulary to freeze in OpenAPI/policy tests. Role names alone never authorize a command.

## Revocation and races

Control membership and session epoch are authoritative. Recheck sensitive operations and queued jobs at execution; recheck package download. Bound any allow-cache staleness explicitly and deny when freshness cannot be established. Context generation detects a stale multi-tab write; clearing frontend caches is necessary but not sufficient. Permission loss cannot undo already downloaded content; signed URLs have a bounded residual exposure window. Stream highly sensitive content if immediate revocation is required.

## Verification

Test login fixation, CSRF, factor/recovery replay, invitation escalation, last-admin concurrency, suspended tenant, same-user multi-tenant cache races, revoked jobs and restricted pricing. An auth feature is not accepted with only successful login tests.
