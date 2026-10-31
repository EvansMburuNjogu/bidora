# BID-ADR-024 — Cell-ready architecture with Hexagonal DDD

Status: **APPROVED — BOUNDED PLATFORM ADMIN REMEDIATION IN PROGRESS**  
Owner: requesting user  
Evidence: “Adopt cell-ready CBA plus Hexagonal DDD, create the bounded architecture decision packet, then refactor Platform Admin boundaries under that combined model.”

## 1. Decision

Bidora adopts two complementary architecture levels:

1. **Hexagonal DDD inside each runtime/control capability** — domain and application own policy and ports; inbound/outbound adapters depend inward; application use cases own abstract transaction orchestration.
2. **Cell-ready architecture at deployment scale** — a global control plane assigns each tenant to one server-owned cell/failure domain; tenant-private effects stay in that assignment; future exchange remains separately governed.

Cell-Based Architecture does not replace module or domain boundaries. A cell is a deployment/isolation boundary, not a Go package, business role, tenant authority or caller-selectable database locator.

## 2. Logical planes

### Global control plane

Owns shared identity, sessions, MFA, Platform Administration, tenant membership/entitlements, tenant-to-cell placement metadata, compatibility and commercial reconciliation. It stores no tenant-private tender drafts, evidence or pricing.

Platform Administrator lifecycle is always control-plane work. It is never routed through a tenant cell and grants no tenant-private access.

### Tenant cells

A cell is a bounded deployment/failure domain containing capacity for a limited set of organizations. A future cell may include cell-restricted API/worker instances, organization databases, cache namespaces, object placement, document/AI workers and event consumers. Each organization still retains its private database.

Every tenant operation resolves placement only after current identity, membership/context and tenant state validation. The server registry returns an assignment plus generation. Caller headers may carry stale-context selectors but never choose a cell, database, cluster, storage key or credential.

### Future exchange plane

Public procurement projections, sealed submission, opening and evaluation remain outside Tenderpreneur cells and require their existing independent policy/security decisions. CBA does not authorize exchange implementation.

## 3. Cell-ready invariants

- One active server-owned cell assignment per tenant generation; no fallback cell.
- Tenant identity and cell identity are distinct; neither is authority.
- API/runtime credentials are cell-scoped and cannot enumerate unrelated cell databases.
- Tenant-private SQL never joins across organizations or cells.
- Cross-cell movement is a durable checkpoint/reconciliation workflow, never a distributed SQL transaction.
- A move uses a new assignment generation, restore/copy validation, authorization reconciliation and approved cutover; stale generations fail closed.
- Events carry provenance and routing facts, not bearer authority; consumers revalidate their assignment and use inbox/effect idempotency.
- Cache, object, vector/search, jobs and provider effects follow the same tenant/cell assignment and are reconciled before cutover.
- Cell failure limits blast radius without changing retained business truth or silently rerouting tenants.
- Control-plane outage and cell outage have separate readiness/degradation behavior; neither may fabricate authorization.
- Compatibility is explicit per control binary, cell runtime and tenant migration set.
- No production region, cell size, tenant count, SLO, RPO/RTO, failover or evacuation policy is selected by this decision.

## 4. Hexagonal module rules

For new Phase 1 modules:

- Domain imports only domain/shared value semantics.
- Application owns use cases, policy sequencing and ports.
- Application transactions use abstract units of work; no pgx, DBTX or sqlc type crosses inward.
- Owner adapters own queries for their tables.
- A same-database transaction may compose multiple owner adapters through a transaction-scoped infrastructure adapter.
- HTTP/provider adapters call application interfaces only.
- Composition roots may know concrete adapters but should consume module facades rather than generated repositories.
- Audit/outbox are transaction-scoped ports and commit with state.

Frozen Option A is not rewritten by this decision. New work follows the stricter boundary, and older modules may be migrated only through separately bounded defect/refactor tickets.

## 5. Platform Admin remediation

BID-ARC-011 applies this decision to the newly added Platform Administration persistence:

- Platform Administration owns governance, profiles, roles, platform invitations, lifecycle changes and idempotency evidence.
- Identity owns user/account-type/auth-epoch/session/MFA queries and effects.
- Authorization owns tenant membership and pending tenant-invitation affiliation checks.
- Platform Admin application services own bootstrap/authority sequencing through abstract control-plane units of work.
- The pgx adapter composes transaction-scoped owner adapters in one control transaction.
- Platform Admin remains global and has no cell assignment dependency.

## 6. Deferred CBA work

This decision does not authorize:

- provisioning cells, databases, Redis, object storage or brokers;
- adding caller-visible cell selectors;
- tenant move/failover handlers;
- changing current deployment credentials or topology;
- production cell sizing, regions or capacity policy;
- cell-specific web behavior;
- starting services or modifying existing databases.

Future implementation requires a bounded tenancy/operations contract with placement, generation, checkpoint, rollback and recovery evidence.

## 7. Verification expectations

Later verification must enforce import direction, owner-query boundaries and composition-only adapter crossings; execute real PostgreSQL rollback and race tests; and prove tenant/cell isolation, stale-generation denial, cell-scoped credentials, object/cache/job confinement and move reconciliation. Documentation or a `cell_id` column alone is not CBA evidence.
