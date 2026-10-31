# BID-FND-003 — Define validated local configuration and isolated service profiles

## Metadata
- Ticket ID: BID-FND-003
- Epic: E-FND
- Type: DevOps
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-002, BID-ARC-007
- Blockers: Tooling/config and environment approval
- Milestone: M1

## 1. Objective
Provide reproducible local dependencies without production secrets or destructive startup.
## 2. Business Context
Tenant routing and file processing must be tested against realistic isolation, not one shared demo database.
## 3. Scope
Configuration schema, safe example names, PostgreSQL control/tenant databases, Compose profiles for NATS/Redis/MinIO/mail/scanner as needed, version pins and explicit seed/migrate/reset commands.
## 4. Out of Scope
Production cloud provisioning, AI containers by default, business migrations or real credentials.
## 5. Technical Requirements
Missing/invalid settings fail startup; local exceptions cannot activate in production. Secrets use ignored files/references. Default startup preserves volumes; reset is separately named and guarded. Expose only necessary local ports.
## 6. Proposed Implementation Approach
Approve environment variable inventory; pin services; separate profiles and credentials; document boot/check/stop; verify unavailable dependency and repeat startup behavior.
## 7. Dependencies
[Toolchain](../architecture/BID-ARC-002.md), [operations baseline](../architecture/BID-ARC-007.md).
## 8. Acceptance Criteria
- Given repeated startup, then data is preserved and no migration is silently run.
- Given production environment with development defaults, then validation rejects unsafe configuration.
- Given optional AI absent, then foundational services remain usable.
## 9. Testing Requirements
Configuration cases, service health/restart, port/secret exposure check and second-engineer clean-start evidence.
## 10. Definition of Done
Documented reproducible profiles and fail-safe validation, without hidden manual setup.
## 11. References
[Operations local](../../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), [deployment](../../technical_design/21_DEPLOYMENT_ARCHITECTURE.md), [environment plan](../../implementation/03_DEVELOPMENT_ENVIRONMENT.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Local resource limits must not imply tested production capacity.

### Approved portable connection-management increment

The user redirected current work away from local-fixture expansion toward server-suitable Go/PostgreSQL, then explicitly said “proceed”. Implemented a standalone infrastructure connection component under `backend/internal/postgres/`, without redefining this parent's broader acceptance or claiming its dependencies Done. [Portable adapter evidence](../../../backend/docs/portable-postgres.md) records the code/configuration contract, pinned dependencies/licenses, tests and remaining gates.

Pinned pgx v5.11.0 with resolved pgpassfile/pgservicefile/puddle/x-sync dependencies. Config is explicit, TLS chain/name verification and SCRAM are mandatory, ambient PG settings are rejected, pools start with zero minimum connections, operations have deadlines, and shutdown cancels/waits without false success. No raw pool/connection is exported; no SQL schema, tenant routing, business operation, server profile or API health contract changed.

`go mod verify`, full Go race tests, vet/build and adapter tests passed on Go 1.27.2; adapter coverage **94.1%**. Linux/amd64 tests cross-compiled only. Synthetic TLS/protocol peers prove rejection boundaries, not actual PostgreSQL success. Govulncheck v1.9.0 with verified Go 1.27.2 scanned 22 modules: no reachable/imported findings; existing unused-openpgp module-only advisory remains.

No PostgreSQL/web service started, server provisioned or deployment performed. Actual successful TLS/SCRAM connection, established-connection pool/shutdown behavior, queries/transactions and reviewed operational acceptance remain pending. Parent status remains Blocked; this isolated code increment is VERIFIED_TECHNICALLY only.

### Backend-only blocker review — 2026-10-17

[Packet 13](../13_BACKEND_FOUNDATION_UNBLOCKING.md) records fresh backend checks and the failed local Docker-daemon query. No PostgreSQL/service profile was implemented or tested. The user subsequently approved packet 13's local-only plan and resource budget. [FND-004](BID-FND-004.md) now records that child, still Blocked on its explicit runtime and technical review gates. This parent stays Blocked, with its current dependencies unchanged; the child does not complete the broader service profile.
