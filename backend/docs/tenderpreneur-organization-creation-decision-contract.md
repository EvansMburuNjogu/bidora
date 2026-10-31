# TENDERPRENEUR-ORG-001 — minimal self-service organization-creation decision contract

Status: **D1–D5 APPROVED / ORG-001–002 IMPLEMENTED_NOT_VERIFIED / UNMOUNTED**  
Owner: requesting user  
Approval evidence: “Approve TENDERPRENEUR-ORG-001 D1-D5 as recommended and proceed with the unmounted contract/schema/application implementation; do not provision databases or call payment providers.”  
Persona: verified Tenderpreneur creating their first Bidora organization  
Operation: durable self-service creation and provisioning of one private Tenderpreneur organization

## 1. Purpose

This packet asks for the minimum policy decisions required before Bidora may implement self-service organization creation. It does not authorize a handler, migration, provisioning worker, database/role creation, payment call, route, frontend change or deployment.

The approved product journey remains:

```text
verified account
→ deliberate Tenderpreneur account type
→ organization basics and commercial choice
→ durable provisioning
→ initial Organization Administrator membership
→ active trial or reconciled paid subscription
→ private workspace access
```

A 202 or provisioning record is pending work, not an active organization.

## 2. Recommended bounded decision

### D1 — Creator authority

**Recommended approval:**

A self-service creator must have all of the following at command time and replay:

- a verified, non-disabled identity;
- irrevocably selected `TENDERPRENEUR` account type;
- a current browser session and exact approved Origin/CSRF protection;
- primary authentication no older than 15 minutes, or current TOTP-backed AAL2 when MFA is enrolled;
- no previously successful or non-reversibly provisioned self-created organization under the pilot lifetime limit;
- no other in-progress organization-creation operation.

No existing tenant membership is required. An account may still hold invited memberships in other organizations. Account type, tenant headers, Platform Administrator status and possession of an idempotency key never grant creator authority.

The command uses a UUID idempotency key and reauthorizes matching replay. Changed payload with the same key conflicts.

### D2 — Initial sole-administrator ownership

**Recommended approval:**

The accepted control-plane request creates exactly one creator membership and the fixed `ORGANIZATION_ADMIN` grant bundle in the same control transaction as the `REQUESTED` tenant and provisioning operation. The membership may exist while the tenant is not active, but grants no usable tenant authority until current tenant state, compatible assignment and membership checks all pass.

The fixed bundle remains exactly the existing source-defined capabilities:

- `tenant.memberships.manage`;
- organization profile read/manage;
- organization letterhead read/manage;
- organization subscription read/manage.

It does not automatically grant Bid Manager, Contributor, Approver, private-bid access or Platform authority.

The creator is the initial and sole effective Organization Administrator. Existing final-administrator continuity applies immediately: they cannot revoke/offboard themselves or lose the final effective administrator grant without an eligible successor and the approved transfer process. The provisioner is a system executor, never an organization member or hidden co-owner.

### D3 — Provisioning checkpoints and rollback

**Recommended approval:**

Provisioning is one durable idempotent operation with server-owned placement and ordered checkpoints:

```text
REQUESTED
→ PLAN_SELECTED
→ PAYMENT_RECONCILED (paid path only)
→ DATABASE_RESERVED
→ DATABASE_CREATED
→ MIGRATIONS_APPLIED
→ RUNTIME_GRANTS_APPLIED
→ COMPATIBILITY_VERIFIED
→ ASSIGNMENT_ACTIVATED
→ SUBSCRIPTION_ACTIVATED
→ ACTIVE
```

The exact commercial ordering differs by the approved path in D5, but every retry resumes the same operation and tenant ID. It never creates a replacement tenant silently.

On failure:

- stop at the failed checkpoint and record only an allowlisted safe failure code;
- tenant access remains fail-closed and no route may use an incomplete assignment;
- retry is allowed only from the last verified checkpoint;
- compensation may remove only resources tagged as owned by that operation and only when non-use and ownership are proven;
- an uncertain database, credential, assignment, migration or payment effect is never deleted, reused or declared rolled back automatically;
- ambiguity moves the operation to `REVIEW_REQUIRED` for an authorized Platform Operations Administrator;
- control/audit/idempotency history is never deleted;
- after assignment activation or any user/business write, automatic database drop is prohibited; recovery is forward-fix or restore to a new target.

Notification or browser disconnect cannot reverse committed provisioning state. Paid-payment ambiguity follows the separately governed commercial reconciliation/reversal policy and must not fabricate a refund or entitlement.

### D4 — Pilot limits and abuse controls

**Recommended approval:**

- one in-progress self-service creation operation per identity;
- one successfully or non-reversibly self-created organization per identity for the pilot;
- invited memberships in other organizations do not count toward that creation limit;
- failed attempts retry the same operation and do not create additional tenant rows;
- identity scope: at most 3 creation requests per 24 hours;
- network scope: at most 10 creation requests per 24 hours;
- denied scopes remain blocked for 15 minutes and return generic `429` with `Retry-After: 900`;
- limiter keys use the approved external HMAC scope secret and fail closed when the limiter is unavailable;
- server-owned cell/cluster capacity must be available before acceptance; no fallback cluster/database is allowed;
- organization name is display data, not a globally unique authority key and not proof of legal ownership.

The lifetime creation limit is consumed only after a resource becomes non-reversibly provisioned or the operation succeeds. A fully compensated failure may continue through the same operation but does not authorize a second concurrent tenant.

### D5 — Trial and paid-subscription behavior

**Recommended approval:**

The creator must explicitly choose `TRIAL` or an exact paid catalogue plan/cadence before provisioning proceeds. Nothing is preselected.

**Trial path:**

- provision first through compatible active assignment;
- then atomically start the existing one-lifetime `PHASE1_V1` 14-day Trial;
- the trial clock begins only when the Trial transaction commits, never when the user submits the request or while provisioning is pending;
- Trial limits are the existing approved immutable catalogue snapshot;
- retry cannot start a second Trial or extend its end time.

**Paid path:**

- create only the existing exact quote/invoice/payment-pending facts;
- do not call a payment provider or charge automatically from organization creation;
- provisioning proceeds only after an authoritative reconciled-payment fact for the exact immutable term;
- paid access activates only after both provisioning compatibility and subscription activation succeed;
- ambiguous payment or activation enters review/reconciliation rather than granting access.

For both paths, tenant-private writes remain denied until current membership/assignment checks pass and commercial access is `FULL`. A provisioned organization without an active Trial or paid term may expose only bounded onboarding/payment status, not tender operations.

## 3. Minimum organization facts

Before the operation may become `ACTIVE`, it must retain the already approved minimum facts:

- registered/display organization name;
- country;
- business type;
- primary industry/category;
- work contact;
- preferred language;
- confirmed currency, timezone and locale.

These are organization profile facts, not proof of incorporation or procurement eligibility. No tax identifier, licence or certification is inferred.

## 4. Transaction and evidence boundaries

- Initial tenant, creator membership/fixed grants, provisioning operation, command hash, audit and outbox facts commit atomically in control.
- Database creation, migrations, grants, compatibility, assignment and commercial activation are durable checkpoints, not a pretend distributed transaction.
- Every checkpoint records initiating creator and system executor separately.
- Tenant database names, credentials, secret references and provider diagnostics are never returned to the browser or written into ordinary audit/outbox payloads.
- Replays recheck current creator eligibility and return the same safe operation identity/state.
- `/v1/me` must not return `/tenderpreneur` from a `REQUESTED`, failed, commercially inactive or incompatible tenant.

## 5. Explicit exclusions

This decision would not approve:

- arbitrary multiple self-created organizations;
- automatic Organization Admin access to bids;
- browser-selected database/cell/cluster identifiers;
- automatic Trial selection, payment, refund or paid activation;
- custom roles or browser-supplied capabilities;
- bypass of final-administrator continuity;
- automatic deletion of ambiguous or used databases;
- production provisioner credentials, secret-manager provider, cluster values or capacity;
- provider acceptance, deployment, UAT or formal release acceptance.

## 6. Approval and implemented intake boundary

The requesting user approved D1–D5 as recommended. The unmounted intake implementation now provides:

- draft `POST /v1/tenderpreneur/organizations` contract with strict 4 KiB JSON and pending-only 202 semantics;
- control migration 31 for creation operations, lifetime markers, provisioning jobs and hash-only commands;
- application policy for verified/non-disabled Tenderpreneur identity, recent assurance, MFA-aware AAL2, fail-closed limiter port, exact validation, one-open/lifetime checks and idempotent replay;
- a pgx/sqlc transaction that creates a `REQUESTED` non-routable tenant, one creator membership, the exact seven-capability Organization Administrator bundle, creation operation/job/command and minimal audit/outbox evidence atomically;
- `TRIAL` jobs start as `PENDING`; `PAID` jobs start as `WAITING_PAYMENT`. Neither path starts a Trial, activates payment/subscription or provisions a resource.

TENDERPRENEUR-ORG-002 additionally provides a strict unmounted HTTP handler and Redis limiter adapter. The handler independently enforces exact session Origin/CSRF, trusted-proxy client IP resolution, UUID idempotency, strict 4 KiB JSON, required/unknown/duplicate-field rejection, safe errors and pending-only 202 output. HMAC-domain-separated identity/network scopes are the only rate keys supplied to Redis. The atomic Redis script allows 3 identity and 10 network requests per 24 hours, applies a 15-minute block after excess and fails closed on dependency or result-shape errors. `BuildTenderpreneurOrganizationCreationHandler` composes the store, application service, limiter and HTTP handler but registers no route.

Still excluded and blocked: route activation, worker/checkpoint transitions, lifetime-marker writes, database/role/secret creation, migrations against a tenant database, payment/provider calls, Trial/subscription activation, runtime migration 31 execution and functional/live verification.
