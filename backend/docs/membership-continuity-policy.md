# Pilot membership continuity and ownership-transfer policy preparation

Status: **APPROVED — IMPLEMENTED_NOT_VERIFIED**  
Prepared: 2026-10-18  
Scope: Tenderpreneur tenant membership offboarding and tenant-local active-work continuity.

## Why this policy is needed

A membership must not be removed while it is the tenant's final effective administrator or while active bid work would become orphaned. Control-plane membership authority and tenant-private work live in different databases, so Bidora cannot honestly claim one atomic transaction across both. The pilot therefore needs a fail-closed, resumable offboarding operation rather than an unrestricted membership update or an automatic reassignment.

## Recommended policy

### 1. Authority and terminology

- A **continuity administrator** is not a role label. It is a verified, non-disabled user with a current `ACTIVE` membership in the current `ACTIVE` Tenderpreneur tenant and an effective `tenant.memberships.manage` `ALLOW` after same-capability `DENY` precedence.
- The actor must be a current continuity administrator with an exact context generation. Tenant IDs, role labels and platform-admin status confer no authority.
- Beginning, cancelling or finalizing offboarding requires an exact approved HTTPS Origin, session-bound CSRF, a session created within the previous 15 minutes, a reason of 1–2,000 characters, expected target context generation and idempotency.
- The pilot introduces no implicit super-admin, break-glass bypass or platform-admin override.

### 2. Last-administrator invariant

- Beginning offboarding, suspending/revoking a membership, or revoking/denying its final effective `tenant.memberships.manage` grant must leave at least one **other** continuity administrator.
- The invariant is evaluated under a lock on the tenant control record and current membership/grant facts, so concurrent removals cannot both pass.
- Self-offboarding is allowed only when another continuity administrator remains and all other conditions below pass.
- Disabled/unverified users, non-`ACTIVE` memberships, inactive tenants, role labels and denied capability facts do not count as successors.
- The same invariant must guard every future membership-state and grant-management path, not only one HTTP handler.

### 3. Explicit successor and transferable work

- Transfer is explicit; Bidora never chooses a successor automatically.
- The successor must differ from the departing user and currently be a verified, non-disabled `ACTIVE` member of the same tenant with effective `tenderpreneur.bid.read` authority.
- While an offboarding operation is active, ordinary control-plane membership/grant mutation paths must refuse to suspend, revoke or remove the required effective read authority from its named successor. Security-driven identity disablement is never blocked. Before tenant transfer it requires a current administrator to select a new eligible successor through a versioned, audited retarget operation. After tenant transfer, completed evidence is never rewritten: an administrator must use non-reversing cancellation and a new offboarding operation for the now-ineligible successor.
- The operation transfers only active tenant-local responsibility:
  - ownership of unarchived bid workspaces;
  - ownership of `OPEN` requirements in unarchived workspaces; and
  - assignment of `OPEN` review threads in unarchived workspaces.
- Archived workspaces and closed/historical requirements, reviews, comments, decisions, approvals, submission evidence, outcomes and audit provenance remain attributed to their historical actors.
- At most 500 affected active records may be transferred in one pilot operation. A larger workload fails before mutation and requires a separately approved batching policy.
- Transfer updates use expected versions and lock affected rows. The tenant transaction records one immutable transfer event plus audit/outbox evidence; no partial tenant transfer is committed.

### 4. Cross-plane offboarding sequence

Because control and tenant databases cannot share a transaction, the operation is a durable fail-closed sequence:

1. **Begin in control:** lock and recheck actor, target, successor and last-administrator facts; create one idempotent offboarding operation; transition the target membership from `ACTIVE` to `OFFBOARDING`; bump context generation; record audit/outbox evidence. `OFFBOARDING` is ineligible for authorization and new assignment.
2. **Fence and transfer in the tenant database:** route only through the server-owned current assignment. Every ownership/assignment mutation must lock a tenant-local per-user assignment guard (a concurrency fence, never an authority source). The offboarding transaction locks and blocks the departing user's guard, then rechecks and transfers all active responsibility atomically to the approved successor and records immutable transfer, audit and outbox evidence. An earlier in-flight assignment finishes before the fence and is included; a later assignment sees the fence and fails closed.
3. **Finalize in control:** recheck the same operation, tenant assignment generation, successor eligibility and last-administrator invariant; revoke all still-active target grants append-only; transition `OFFBOARDING` to `REVOKED`; bump context generation and record audit/outbox evidence.

A failure after step 1 leaves the target fail-closed in `OFFBOARDING`; retries resume from the durable checkpoint. A failure after step 2 leaves work safely assigned to the successor while the target remains unauthorized until finalization. Neither case silently restores authority or repeats transfer effects.

### 5. Cancellation and recovery

- An `OFFBOARDING` operation may be explicitly cancelled by a current continuity administrator using fresh authentication, expected operation version, reason and idempotency.
- Cancellation returns the membership to `ACTIVE` only after rechecking identity/tenant eligibility and the last-administrator invariant, clears the tenant assignment fence through a durable checkpoint, bumps context generation, and records audit/outbox evidence.
- Cancellation never reverses a completed tenant ownership transfer; historical transfer evidence remains immutable.
- A `REVOKED` membership cannot be restored by this pilot policy. Re-entry requires a new separately governed invitation/reactivation policy.

### 6. Reads, privacy and response semantics

- The existing ownership-continuity read remains the preflight view and must identify the target's active-work counts and whether another continuity administrator exists.
- Responses use internal IDs and bounded counts only. They expose no role labels as authority, private bid content, email in tenant-plane evidence, database locator or credential material.
- Safe conflicts distinguish only stable conditions such as `LAST_ADMIN_BLOCKED`, `SUCCESSOR_INELIGIBLE`, `ACTIVE_WORK_CHANGED`, `OFFBOARDING_IN_PROGRESS` and `TRANSFER_LIMIT_EXCEEDED` to already-authorized managers.
- `202` means the cross-plane operation is pending, not that membership revocation or transfer completed. Only a terminal `COMPLETED` read may report revocation complete.

## Proposed contracts

- `POST /v1/shared/memberships/{membership_id}/offboarding` — begin/resume with `successor_membership_id`, reason, expected target context generation and idempotency; returns `202` unless already terminal.
- `GET /v1/shared/membership-offboarding/{operation_id}` — current authorized checkpoint/status.
- `POST /v1/shared/membership-offboarding/{operation_id}/retarget` — replace an ineligible successor using expected operation version and new eligibility checks.
- `POST /v1/shared/membership-offboarding/{operation_id}/cancel` — explicit pre-finalization cancellation.

The existing narrow capture-contributor deletion remains valid only for a target whose sole active grant is the fixed capture-contributor grant and whose continuity report has zero active ownership/assignments. It must not become a general administrator-removal shortcut.

## Required implementation if approved

- Control migration: add `OFFBOARDING`, durable offboarding operations/events and last-administrator-safe state/grant transition queries.
- Tenant migration: per-user assignment guards, immutable ownership-transfer events and versioned bulk transfer queries; all current assignment mutations must participate in the guard protocol.
- Application orchestration: server-owned routing, durable checkpoints, retry/idempotency, explicit cancellation and concealed safe errors.
- Contracts: strict request/response schemas and pending-versus-completed semantics.
- Deferred verification: concurrent final-admin removals, assignment races, cross-plane crash points, idempotent resume/cancel, tenant isolation and live migration evidence.

## Explicit exclusions

No automatic successor selection, role catalog, arbitrary grant editor, platform override, ownership transfer across tenants, reassignment of immutable history, account deletion, personal-data erasure, archive restoration, invitation/reactivation policy, deployment or release acceptance is included.

## Approval evidence

The requesting user approved BID-ADR-021 on 2026-10-18 with the capability-derived administrator definition, 15-minute fresh-session requirement, `OFFBOARDING` fail-closed state, explicit successor requirements, exact transferable record set, 500-record bound, resumable cross-plane sequence, audited retargeting, non-reversing cancellation and no pilot restoration or override. Source implementation remains `IMPLEMENTED_NOT_VERIFIED`; migration execution, race/crash verification, security review and UAT are deferred.
