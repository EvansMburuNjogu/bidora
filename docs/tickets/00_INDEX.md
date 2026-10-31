# Bidora engineering tickets

> **Historical ticket index.** Current full-stack scope, execution policy and the five active parent tickets live in [Platform Admin delivery](PLATFORM_ADMIN_DELIVERY.md) and [root instructions](../../AGENTS.md). Dated permission limits, status totals and approval procedures below are historical evidence, not current development gates.


## Purpose and current authorization

This backlog was created as documentation-only planning. A subsequently authorized local Go ↔ Next.js health slice is now implemented and technically verified; see [execution evidence](12_HEALTH_SLICE_EVIDENCE.md) and [startup commands](../../README.md). Other tickets remain plans, not implemented business software. The original backlog-generation task changed only `docs/tickets/`. Subsequent authorized decision synchronization also updated BID-ADR-006 in the technical-design register and the implementation strategy. The bounded health implementation and its dependencies were subsequently authorized. Business features, database/service provisioning, production deployment, Git initialization and product-source editing remain outside that scope.

Approved roots (BID-ADR-005): `web/` and `backend/`, now containing the local health slice; `frontend/` remains absent and must not be created as a duplicate. BID-ADR-008 approves Goose and npm; exact version verification remains pending.

The user subsequently approved [packet 13's local-only backend foundation plan](13_BACKEND_FOUNDATION_UNBLOCKING.md), with 2 CPUs, 2 GiB RAM and a 10 GiB storage budget. [FND-004](foundation/BID-FND-004.md) records the bounded child; runtime, exact-pin/security and contract-review blockers remain. This does not approve production, identity or procurement policy, and no web work is authorized.

## Navigation

1. [Epics and capability coverage](01_EPICS.md)
2. [Master backlog: all 110 individual tickets](02_MASTER_BACKLOG.md)
3. [Dependency graph and critical chains](03_DEPENDENCY_GRAPH.md)
4. [Vertical-slice delivery roadmap and legacy task mapping](04_DELIVERY_ROADMAP.md)
5. [Release milestones and acceptance gates](05_RELEASE_MILESTONES.md)
6. [Ticket standards, Ready and Done](06_TICKET_STANDARDS.md)
7. [Backlog validation and limitations](07_BACKLOG_VALIDATION.md)
8. [Ticket execution runbook](08_EXECUTION_RUNBOOK.md)
9. [BID-ARC-001 release scope decision packet — Option A approved; closure steps pending](09_RELEASE_SCOPE_REVIEW.md)
10. [BID-ARC-002 repository/toolchain decision packet — tools approved; exact version review pending](10_FOUNDATION_DECISION_PACKET.md)
11. [BID-API-001 original foundation health/error proposal](11_FOUNDATION_API_CONTRACT.md)
12. [Local health-slice execution evidence and remaining gates](12_HEALTH_SLICE_EVIDENCE.md)
13. [Backend foundation blocker review and proposed local-only authorization](13_BACKEND_FOUNDATION_UNBLOCKING.md)
14. [Phase 1 Tenderpreneur and Platform Admin gap audit](14_PHASE_1_PORTAL_GAP_AUDIT.md)

## Categories

| Category | Epic | Count | Entry ticket |
|---|---|---:|---|
| architecture | E-ARC | 11 | [Release alignment](architecture/BID-ARC-001.md) |
| foundation | E-FND | 5 | [Go health boundary](foundation/BID-FND-001.md) |
| backend_contracts | E-API | 12 | [OpenAPI conventions](backend_contracts/BID-API-001.md) |
| database | E-DB | 8 | [Migrations/roles](database/BID-DB-001.md) |
| backend | E-BE | 17 | [Command transactions](backend/BID-BE-001.md) |
| authentication | E-AUTH | 8 | [Verified identity](authentication/BID-AUTH-001.md) |
| tenancy | E-TEN | 4 | [Provisioning](tenancy/BID-TEN-001.md) |
| events_workers | E-EVT | 2 | [Outbox/inbox](events_workers/BID-EVT-001.md) |
| documents | E-DOC | 3 | [Immutable storage](documents/BID-DOC-001.md) |
| frontend | E-FE | 11 | [Auth/workspace shell](frontend/BID-FE-001.md) |
| integrations | E-INT | 4 | [Notifications](integrations/BID-INT-001.md) |
| ai_search | E-AI | 5 | [AI contracts/policy](ai_search/BID-AI-001.md) |
| security | E-SEC | 4 | [Threat model](security/BID-SEC-001.md) |
| qa | E-QA | 6 | [Real-dependency harness](qa/BID-QA-001.md) |
| devops | E-OPS | 3 | [CI](devops/BID-OPS-001.md) |
| observability | E-OBS | 2 | [Safe logging](observability/BID-OBS-001.md) |
| release | E-REL | 5 | [MVP acceptance](release/BID-REL-001.md) |

## Active-count reconciliation

As of 2026-10-20, the backend-only Phase 1 queue contains **25 remaining work packets**: 13 functional umbrellas, 10 frozen Option A verification packets and 2 Phase 1 acceptance tickets. The exact basis and exclusions are recorded in [the Phase 1 gap audit](14_PHASE_1_PORTAL_GAP_AUDIT.md#2-reconciled-active-backend-count--2026-10-20). The 110-file inventory and its original status totals are historical planning metadata, not the active remaining count.

## Authority and readiness

Read [AGENTS](../../AGENTS.md), [product index](../product/00_MASTER_INDEX.md), [technical index](../technical_design/00_INDEX.md), [implementation index](../implementation/00_INDEX.md), [canonical decisions](../technical_design/27_ARCHITECTURE_DECISIONS.md), and [design quality report](../technical_design/QUALITY_REPORT.md).

Approved architecture is Go/Fiber modular monolith, Next.js/TypeScript/Tailwind/shadcn, PostgreSQL/pgx/sqlc database-per-organization, NATS JetStream, Redis as ephemeral cache, S3 and separate Python/FastAPI/Pydantic AI when authorized. Existing design documents are not proof of a running system; only the bounded health-slice report records executed runtime verification. Goose and npm have since been explicitly approved. Production providers, same-origin deployment details and business policies must not be inferred approved from recommendations.

There are **8 Proposed, 89 Blocked, 12 IMPLEMENTED_NOT_VERIFIED and 1 In Review tickets**, no Ready/Done tickets. The user subsequently chose manual execution instead of subagents. [FND-005](foundation/BID-FND-005.md), a Docker-free resource-validation helper using only the existing Go toolchain, is implemented and technically verified, awaiting user review. The PostgreSQL integration child remains blocked. Start owner review with ARC-001/002/003/005/006/007/008; ARC-004 follows identity/domain decisions. ARC-009 is future crypto/temporal discovery, not an MVP dependency. Implementation authorization remains a separate requirement after decisions close.

The user-approved Option A increment is manual tender capture through immutable approval/package, external proof and outcome; BID-ADR-006 now records that approval. Remaining reviewer evidence is pending. This is **not** a claim to satisfy all BRD/PRD acceptance: AI search, full procurement, sealed exchange, billing and enterprise expansion retain explicit future gates. See [scope and source mapping](04_DELIVERY_ROADMAP.md).
