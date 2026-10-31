# BID-AUTH-004 / BID-API-010S — Support session, evidence and fencing decision set

Status: **DECISION SET PRESENTED — APPROVAL PENDING; IMPLEMENTATION NOT AUTHORIZED**  
Depends on: approved T1–T6, D1–D6 and B1 in `controlled-support-access-review.md`  
Scope: SS1–SS6, E1–E5 and R1–R5 only  
Excludes: capability identifier selection, schema, migration, API/OpenAPI, token issuance, tenant reads, runtime, tests, frontend and deployment

## 1. Important correction

The earlier SS1 recommendation to use `__Host-bidora_support` with a narrow `/v1/support-diagnostics` cookie path is invalid. A conforming `__Host-` cookie must have `Path=/`, be Secure and have no Domain attribute.

**Recommendation:** retain `__Host-bidora_support` with Secure, HttpOnly, SameSite=Strict, `Path=/` and no Domain. Only exact support-diagnostic middleware may consume it. Unrelated handlers must neither interpret nor log it. Cookie path is request-delivery scoping, not an authorization boundary; server-side session binding and per-action checks remain mandatory.

## 2. Session baseline confirmation

The prior selective approval did not approve the session decisions, so the numeric S5 baseline also remains proposed even though it is not a separately numbered SS item:

- request expires 24 hours after creation;
- approval activation window is 15 minutes;
- tenant selects 15, 30 or 60 minutes, never more than requested;
- absolute lifetime is at most 60 minutes from activation and never beyond grant expiry;
- idle timeout is ten minutes;
- no renewal, extension, refresh or resurrection;
- database time is authoritative and uncertainty denies access.

**Recommendation:** approve this baseline with SS1–SS6. A continuing investigation requires a new request and tenant approval.

## 3. SS1–SS6 — session decisions

### SS1 — cookie boundary

Choose:

1. **Recommended:** `__Host-bidora_support`; Secure; HttpOnly; SameSite=Strict; `Path=/`; no Domain. Only exact support-diagnostic middleware consumes it.
2. Rename to `__Secure-bidora_support` and use a narrow path. This loses the browser-enforced `__Host-` no-Domain/root-path invariant and is not recommended.

Activation/end/revoke use the ordinary Platform session and do not need the support token as authority. Diagnostic reads require the current ordinary Platform session, exact support cookie, exact Origin and server-selected session context. The selector header is never bearer authority.

### SS2 — session concurrency

Choose:

1. **Recommended:** one ACTIVE session globally per Support Administrator and one per approved request. Concurrent activation has one winner. The actor must end the current session before activating access for another tenant.
2. Permit multiple tenant sessions. This increases tab/context confusion and requires an additional user-interface and token-partitioning policy; it is not recommended for Phase 1.

There is no multi-tab or same-tenant exception to option 1.

### SS3 — idle touch and activity

Choose:

1. **Recommended:** advance `last_authorized_activity_at` only when the final control-plane `RELEASE_AUTHORIZED` decision commits. Failures, `READ_PREPARED`, concealed not-found, final denial, case/history reads and dependency errors do not extend idle time. Database time is used, and touch cannot extend absolute/grant expiry.
2. Touch when tenant preparation commits. This can extend a session even when final release is denied and is not recommended.
3. Touch at request arrival. This lets failed probes keep a session alive and is rejected.

A crash after release authorization may touch idle time even if the HTTP response is not received; evidence truthfully records authorization, not receipt.

### SS4 — ordinary-session and assurance binding

**Recommended:** approve all of the following:

- bind support session to exact ordinary Platform session UUID, support user, auth epoch, platform context generation and current Support Administrator assignment;
- ordinary-session rotation/revocation, auth-epoch/context change, identity disablement, role/capability loss or TOTP-factor disablement invalidates support authority;
- a replacement ordinary session never adopts the support session;
- activation requires TOTP AAL2 no older than 15 minutes;
- during the bounded support session, every action rechecks the active TOTP factor and bound auth/session facts but does not require a new TOTP prompt unless ordinary assurance policy independently does so;
- no support token may create a tenant session, membership or capability.

Choose whether a fresh TOTP prompt must also occur on every diagnostic read. **Recommendation:** no; activation freshness plus a maximum 60-minute non-renewable session, current-factor checks and exact ordinary-session binding are proportionate for the approved minimized read-only DTOs. Future content/export/mutation scope must reconsider this.

### SS5 — synchronous-only authority

**Recommended:** a support session authorizes only synchronous, bounded, allowlisted diagnostic reads. It cannot enqueue work, trigger retries, invoke AI/providers, send private notifications, create exports/downloads/signed URLs or authorize a delayed read. Returned DTOs and action permits are not reusable credentials. A request cancelled by context timeout returns no DTO and does not deliberately extend or release authority.

Alternative asynchronous authority is rejected for Phase 1 because execution could occur after revocation, movement or expiry.

### SS6 — token lifecycle and redaction

**Recommended:** approve all of the following:

- 32 random bytes from the cryptographic generator;
- domain/version-separated SHA-256 at rest; raw token returned once;
- accept only the exact cookie, never query/body/URL/generic Authorization header;
- middleware, access logs, tracing, panic recovery and error reporting redact the cookie before diagnostics;
- constant-shape generic failure for absent/malformed/unknown tokens before tenant lookup;
- no token/hash in audit, outbox, notifications, DTOs or selectors;
- explicit end/revoke clears the cookie best effort, while server state remains authoritative;
- no rotation, renewal or token recovery; a new approved request/session is required.

Exact cryptographic domain string is an implementation-contract detail to freeze before source work, not a reason to weaken these semantics.

## 4. E1–E5 — evidence decisions

### E1 — truthful action states

Choose:

1. **Recommended:** use `READ_PREPARED`, `RELEASE_AUTHORIZED` and `RELEASE_DENIED`. Transport completion may be restricted operational metadata but is never proof of human receipt/viewing.
2. Record `VIEWED`, `RECEIVED` or `DELIVERED` from an HTTP write. This is inaccurate and rejected.

`READ_PREPARED` means the owner-plane read and pre-disclosure evidence committed. `RELEASE_AUTHORIZED` means the final control transaction linearized authorization and permit consumption. It does not mean the browser received, rendered or retained bytes.

### E2 — cross-plane evidence sequence

Choose:

1. **Recommended:** a durable correlated sequence:
   1. control creates a one-action pending permit after current authorization;
   2. the owner plane obtains one immutable DTO snapshot and commits `READ_PREPARED`, binding permit ID, policy/DTO version, safe target, actor/case/session references and a domain-separated DTO hash;
   3. control receives the preparation reference/hash and atomically performs the final current authorization check, consumes the permit and records `RELEASE_AUTHORIZED` or `RELEASE_DENIED`;
   4. only `RELEASE_AUTHORIZED` permits serialization of the already-bound DTO snapshot;
   5. append-only reconciliation links ambiguous/crash outcomes without rewriting either plane.
2. Hold tenant SQL open while calling control. This violates the transaction boundary and is rejected.
3. Return after tenant evidence without final control fencing. This leaves a revocation gap and is rejected.

No SQL transaction spans databases or external calls. A prepared-but-not-released event is valid evidence, not disclosure. A finalization ambiguity returns no DTO even if later reconciliation finds authorization committed.

For a control-origin DTO, control owns the immutable source snapshot/hash while the exact tenant owner plane commits the tenant-visible `READ_PREPARED` reference before final release. No document may claim that the tenant SQL transaction atomically read control data.

### E3 — tenant-visible actor and purpose data

**Recommended:** tenant-visible history shows stable support actor display-name/role snapshots and an opaque actor ID, case reference, approved minimized purpose, scope/target references, approver, timestamps and safe result. It excludes email, IP/device/session data, raw token/hash, security telemetry and provider/database diagnostics.

The display/role snapshot is immutable evidence. Purpose is restricted case material, not copied into general audit/outbox/notifications; those carry a purpose hash and safe codes only.

Alternative current-profile resolution is not recommended because later profile changes would rewrite the apparent historical actor.

### E4 — retention and export boundary

**Recommended:** evidence is append-only and unavailable for automated deletion, anonymization or bulk export until R1–R13 retention/legal-hold and privacy rules approve an exact retention/disposition class. Tenant history is bounded/keyset-paginated viewing, not an export. Protective holds take precedence. This is a fail-closed interim retention boundary, not approval for indefinite production retention or release readiness.

No support implementation may activate in production until lawful purpose, retention class and disposition evidence are approved.

### E5 — denied and failed attempts

Choose:

1. **Recommended:**
   - malformed/forged/pre-session probes remain restricted low-content platform security telemetry and do not create attacker-controlled tenant records;
   - once a valid active session and tenant scope are established, out-of-scope, concealed-not-found and final-denied actions append safe tenant-visible denial evidence without echoing the probed private identifier or revealing existence;
   - dependency failures record only a stable safe class and no raw diagnostics;
   - repeated valid-session reads create distinct action attempts.
2. Record every unauthenticated probe in tenant evidence. This permits tenant-log pollution and is rejected.
3. Hide all denials from tenants. This weakens accountability and is not recommended.

## 5. R1–R5 — revocation and fencing decisions

### R1 — authoritative single-action permit and race ordering

Choose:

1. **Recommended:** control owns a single-use action permit and finalization fence:
   - permit binds exact grant/session version, actor, tenant assignment generation, scope, target, action, owner policy/DTO version and random permit UUID;
   - permit lifetime is 15 seconds, never beyond idle/absolute/grant expiry;
   - owner preparation has a ten-second deadline and binds the DTO hash/evidence ID;
   - permit creation and finalization each recheck authoritative control state;
   - finalization and revocation lock/advance the same authoritative grant/session generation, giving one database commit order;
   - revocation invalidates all unfinalized permits and never waits for them;
   - if finalization commits first, that one action is `RELEASE_AUTHORIZED`; later revocation cannot recall it;
   - if revocation commits first, finalization is `RELEASE_DENIED` and no DTO bytes are returned;
   - a permit is internal authorization state, never a browser token or owner of continuing authority;
   - missing, expired, replayed, mismatched or ambiguous permits fail closed.
2. Use only asynchronous tenant projection. This has a revocation lag and cannot satisfy no-cache-grace semantics.
3. Hold a control transaction open across tenant work. This violates bounded transaction rules and is rejected.

The guarantee is precise: no action may linearize release authorization after revocation commits. It does not falsely claim recall after an earlier release-authorization commit or prove browser receipt.

The 15-second permit and ten-second owner deadline are recommended Phase 1 limits. Approval may select tighter values but must not permit defaults or values beyond session/grant expiry.

### R2 — authoritative revokers

**Recommended:** approve only:

- any current effective tenant support approver for the exact tenant, with recent TOTP and ordinary tenant authority;
- the exact Support Administrator ending their own session;
- current Support or Security/Audit Administrator with explicit `platform.support.session.revoke`, recent TOTP and current Platform context;
- automatic authoritative invalidation from the approved session/grant conditions.

Platform Owner, account type, deployment operator, routing metadata or support-session possession grants no implicit revocation authority. Revocation cannot delete or rewrite history.

### R3 — closure, authority loss and invalidation

**Recommended:** case closure, tenant/identity suspension, original approver authority loss, support-role/capability/TOTP loss, ordinary-session/auth-epoch/context change, expiry and integrity conflict atomically advance authoritative control eligibility/generation and invalidate pending permits. Another approver/support actor never inherits the request/session. Continuing work requires a new eligible case/request/approval/session.

Projection reconciliation follows; control permit finalization remains authoritative for release. No asynchronous projection is represented as immediate completion.

### R4 — tenant movement and dependency uncertainty

**Recommended:** assignment-generation change denies new permits and invalidates old-generation sessions. No request, permit, DTO or evidence action is forwarded to a replacement cell. After separately approved movement reconciliation, continuing support requires a new case/request/approval against the current assignment.

Control/tenant dependency uncertainty, incompatible schema, deadline expiry or ambiguous finalization returns no DTO, creates no fabricated success and never extends the session. Retry is a new evidenced action only while the original session remains independently valid.

### R5 — revocation evidence, projection and notification

**Recommended:** control commit is the authoritative revocation time. Tenant and support reads show control status immediately where available and separately label tenant projection/checkpoint state as pending, acknowledged or held. No response claims global reconciliation until exact acknowledgements exist.

Revocation appends immutable actor, authority source, reason code, safe reason hash/text under its classification, affected grant/session generation and timestamp. Notifications are generic best effort and never authority or completion evidence. Failure to notify cannot preserve access. Projection mismatch or generation-ahead facts enter integrity hold and deny release.

## 6. Combined recommended selection

Approve:

- the numeric session baseline;
- SS1 option 1, SS2 option 1, SS3 option 1, SS4 as recommended including no per-read TOTP reprompt, SS5 and SS6 as recommended;
- E1 option 1, E2 option 1, E3 and E4 as recommended, E5 option 1;
- R1 option 1 with a 15-second permit and ten-second owner deadline, and R2–R5 as recommended.

This selection would approve a policy/contract direction only. It would not freeze the exact tenant capability identifiers and would not authorize implementation, test execution, migration, route mounting, tenant access, runtime activation or deployment.

## 7. If approved, remaining pre-implementation decisions

Before any separately authorized source work:

1. freeze exact tenant capability identifiers from T1;
2. freeze support policy version, DTO versions and safe failure-code registries;
3. define exact control/tenant aggregate ownership and command schemas without cross-database transactions;
4. obtain retention/lawful-purpose evidence required by E4;
5. approve a separately bounded implementation slice.

No implementation was performed by this decision set.
