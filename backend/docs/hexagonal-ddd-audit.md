# Backend Hexagonal DDD architecture audit

Status: **R1–R4 APPROVED AND SOURCE-IMPLEMENTED FOR PLATFORM ADMIN; R5/TRANSACTION EVIDENCE DEFERRED**  
Audit ID: `ARC-HEX-001`  
Scope: active Go backend, with detailed review of BID-AUTH-003B Platform Administration  
Method: source/import inspection and package dependency analysis only; no runtime, database or test execution.

## 1. Verdict

Bidora uses important **Ports and Adapters / Clean Architecture dependency rules**, but it is not yet consistently implemented as a hexagonal DDD modular monolith.

The accurate description is:

> **A modular, port-oriented layered monolith with substantial hexagonal structure, but inconsistent transaction and bounded-context ownership.**

The framework boundary is healthy: domain/application packages do not import Fiber, pgx, Redis, MinIO or generated repositories, and HTTP handlers do not call sqlc directly. The main conformance gaps are that persistence adapters own most transaction orchestration and business branching, several adapters query another bounded context's tables, and module boundaries are not enforced automatically.

The requesting user subsequently approved R1–R5 together with cell-ready CBA under BID-ADR-024. BID-ARC-011 corrects the new Platform Admin bootstrap/authority source boundary; HTTP and broader lifecycle work remain gated by deferred R5 checks and real PostgreSQL transaction evidence.

## 2. Audit criteria and results

| Hexagonal/DDD criterion | Result | Evidence |
|---|---|---|
| Domain is framework independent | **PASS** | Domain packages have no Fiber/pgx/provider imports. `platformadmin/domain` imports only `errors`. |
| Application depends inward and owns ports | **PASS, structurally** | Application packages expose repository/reader/committer interfaces; static inspection found no application imports of Fiber, pgx, Redis, MinIO, persistence or HTTP packages. |
| Inbound HTTP is an adapter | **PASS** | `internal/httpapi` imports application APIs and has no sqlc/pgx/PostgreSQL transaction use. |
| Outbound persistence depends on application/domain | **PASS** | Persistence packages import application/domain ports and isolate generated sqlc. |
| Composition is outside domain/application | **PASS, with maintainability debt** | `internal/pilot` composes concrete adapters, but `runtime.go` is 998 lines and imports generated sqlc packages directly for compatibility wiring. |
| Application owns use-case transaction orchestration | **FAIL** | Static count found 92 `WithinTransaction` uses in persistence adapters and zero in application packages. Persistence methods commonly lock, authorize state, apply limits, write state/audit/outbox and choose domain errors. This conflicts with `08_BACKEND_ARCHITECTURE.md` and AGENTS.md. |
| Bounded contexts own their tables/adapters | **FAIL/PARTIAL** | New `queries/platformadmin/lifecycle.sql` directly reads/updates Identity tables, Tenancy membership/invitation tables, sessions and MFA tables. Existing Opportunity persistence also imports Authorization application types. This weakens independent module ownership. |
| Domain behavior is concentrated in aggregates/value objects | **PARTIAL** | Opportunity has explicit domain values; many identity/authorization invariants live in application or persistence transaction methods. Platform roles are clean domain values, but bootstrap eligibility and continuity decisions currently reside in SQL/persistence. |
| Wire, domain and persistence models are separate | **PASS/PARTIAL** | Most handlers map application DTOs and sqlc remains adapter-local. Some application commit structures are persistence-shaped and several stores return application errors based on row-level branching. |
| Architecture dependency rules are executable | **FAIL** | The only active architecture test protects retired tooling. There is no import rule preventing application→adapter, handler→sqlc or cross-bounded-context persistence dependencies. Test implementation remains deferred under the current development phase. |
| Module layout communicates ports/adapters clearly | **PARTIAL** | `domain`, `application` and `persistence` are present for some modules, including Platform Admin. Provider adapters are often module-root siblings (`attemptredis`, `deliverysmtp`, `extractopenai`) and all inbound HTTP lives in one 49-file `httpapi` package. Naming is workable but inconsistent. |

## 3. Platform Administration findings

### 3.1 Conformant parts

- `internal/platformadmin/domain` owns fixed role/capability values and has no infrastructure dependency.
- `internal/platformadmin/application` defines ports and redacted application types.
- `internal/platformadmin/persistence/sqlc` contains generated database details.
- `internal/platformadmin/persistence` depends inward on application/domain.
- The HTTP contract does not grant authority and no handler directly accesses persistence.
- The migration owns a dedicated `platform_administration` schema and does not store raw tokens or provider secrets.

### 3.2 Original blocking boundary issue: cross-context table access

At audit time, `queries/platformadmin/lifecycle.sql` directly accessed:

- `identity.users`, `identity.sessions` and `identity.mfa_factors`;
- `tenancy.memberships` and `tenancy.invitations`;
- shared audit and outbox tables.

This makes the Platform Administration persistence adapter simultaneously an Identity, Authorization/Tenancy, Session, MFA, Audit and Eventing adapter. It bypasses owner-module application ports and conflicts with the repository rule that only an owner module queries its tables.

The database foreign keys from Platform Administration to immutable Identity IDs may remain a reviewed integrity dependency. Direct lifecycle reads and updates should not remain in the Platform Administration query owner without an explicit bounded-context ownership decision.

### 3.3 Original blocking transaction issue: persistence owned the bootstrap use case

At audit time, `platformadmin/persistence.Store.CommitBootstrap` performed the entire use case:

- decides bootstrap consumption and eligibility;
- checks owner history;
- promotes account type and auth epoch;
- revokes sessions;
- creates profile and role assignment;
- consumes governance state;
- creates audit and outbox records.

It is atomic, but atomicity alone is not hexagonal application orchestration. The adapter owned business sequencing and mapped policy outcomes. This remains an established pattern in many older stores, but it does not match the approved target architecture and has now been corrected for the new Platform Admin bootstrap path.

### 3.4 Remaining non-blocking organization debt

- `internal/pilot/runtime.go` is a large manual composition root and knows generated sqlc constructors.
- `application/ports.go` combines port interfaces and DTOs in one file. This is acceptable at current size but should split by use case as behavior grows.

## 4. Recommended bounded remediation

Do not broadly rewrite existing Option A modules. Apply a strangler rule to new Phase 1 work and repair only BID-AUTH-003B before building upward.

### R1 — Establish explicit Platform Administration ownership

Treat `platformadmin` as the use-case owner for Platform Administrator lifecycle, while Identity owns user/account/session/MFA writes and Authorization/Tenancy owns membership/invitation affiliation reads.

Platform Administration may own:

- governance state;
- administrator profiles;
- platform role assignments;
- platform administrator invitations;
- administrator lifecycle proposals/decisions;
- operation idempotency evidence.

It must not own direct queries that mutate Identity or inspect Tenancy affiliation.

### R2 — Add an abstract application unit of work

The application layer should own bootstrap and lifecycle sequencing through an abstract control-plane unit of work, for example:

```go
type BootstrapUnitOfWork interface {
    Within(ctx context.Context, fn func(BootstrapTx) error) error
}

type BootstrapTx interface {
    LockGovernance(context.Context) (Governance, error)
    GetIdentityEligibility(context.Context, ID, time.Time) (IdentityEligibility, error)
    GetAffiliationEligibility(context.Context, ID, time.Time) (AffiliationEligibility, error)
    PromotePlatformIdentity(context.Context, Promotion) (IdentityEffect, error)
    CreateOwner(context.Context, OwnerBootstrap) error
    AppendAudit(context.Context, AuditFact) error
    AppendOutbox(context.Context, EventFact) error
}
```

The interface exposes business operations, not `pgx.Tx`, `postgres.DBTX`, sqlc rows or table names. The pgx adapter supplies transaction-scoped implementations.

### R3 — Return foreign-table SQL to owner adapters

- Move account type/auth-epoch/session/MFA queries behind Identity-owned transaction-scoped adapters.
- Move membership and pending tenant-invitation checks behind Authorization/Tenancy-owned adapters.
- Keep role/governance/invitation/change queries in Platform Administration.
- Use shared control-transaction composition to preserve one PostgreSQL transaction without exposing pgx to application code.
- Keep audit/outbox append operations as transaction-scoped ports.

### R4 — Add a module facade before HTTP

Expose Platform Admin application constructors/use cases from its module boundary. Fiber handlers should depend only on those interfaces. `pilot` should compose the facade and should not import Platform Admin generated sqlc.

### R5 — Add dependency regression checks during the verification phase

Without adding tests now, reserve later centralized architecture checks for:

- domain/application cannot import adapters or external infrastructure SDKs;
- HTTP cannot import persistence/sqlc;
- generated sqlc is imported only by its owning persistence adapter;
- cross-module persistence imports/table ownership require an allowlisted architecture decision;
- composition roots are the only packages allowed to import inbound and outbound adapters together.

## 5. Decision and bounded remediation outcome

The requesting user approved R1–R5 and adopted cell-ready CBA plus Hexagonal DDD. [BID-ADR-024](cell-ready-hexagonal-architecture.md) records the combined model.

BID-ARC-011 source implementation now:

- moves Identity lifecycle/session/MFA SQL into `queries/identity/platform_administration.sql`;
- moves membership/pending-invitation affiliation SQL into `queries/authorization/platform_administration.sql`;
- moves shared audit/outbox appends into `queries/controlplane/evidence.sql`;
- leaves `queries/platformadmin/lifecycle.sql` referencing only Platform Administration-owned tables;
- adds application-owned bootstrap and authority services using abstract control-plane units of work;
- composes transaction-scoped owner adapters in the outer pgx persistence adapter; and
- adds `internal/platformadmin/module.go` as the inbound module facade.

R1–R4 are `IMPLEMENTED_NOT_VERIFIED`. R5 remains deferred to the verification phase, as do real PostgreSQL single-winner, rollback, owner-continuity and evidence atomicity checks. No decision changes frozen Option A implementation status or authorizes a repository-wide refactor, HTTP activation, cell infrastructure or migration execution.

## 6. Evidence and limitations

Static evidence collected:

- 57 active backend packages after Platform Administration generation;
- 54 application port/repository/reader/committer/unit-of-work interface declarations;
- zero detected application imports of Fiber, pgx, Redis, MinIO, persistence or HTTP packages;
- zero detected HTTP imports of sqlc/pgx/PostgreSQL transaction types;
- 92 `WithinTransaction` occurrences across persistence adapters and zero in application packages;
- 998-line `internal/pilot/runtime.go` and 49-file shared `internal/httpapi` adapter;
- no active module-level `module.go` facade files;
- one active architecture test, currently limited to retired-tooling separation.

These counts are structural indicators, not runtime correctness or a complete semantic proof. No test body, database, service, migration or provider was executed during the original audit.

Post-remediation static/compile checks on 2026-10-20 found 64 active backend packages, zero Platform Admin domain/application infrastructure imports, zero foreign-owner schema references in `queries/platformadmin/lifecycle.sql`, zero sqlc generation drift, 95 matching reviewed source checksums and successful `go build ./...` plus compile-only `go test -run '^$' ./...`. These checks establish source shape and compilability only; R5 semantic dependency tests and live PostgreSQL transaction/race/rollback evidence remain deferred.
