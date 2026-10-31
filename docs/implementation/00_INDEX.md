# Bidora implementation documentation index

## 1. Purpose
Guide future authorized engineering from specifications to verified external-tender MVP. Nothing in this collection authorizes application development by itself.

## 2. Scope
Phases 0–9 as workstreams, delivered through vertical slices; later AI and sealed marketplace are gated separately.

## 3. Product specifications
Start with [master index](../product/00_MASTER_INDEX.md), [module dependencies](../product/07_MODULE_DEPENDENCIES.md), [delivery](../product/17_DELIVERY_PLAN_WAVES_SPRINTS.md) and [evidence](../product/19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md).

## 4. Related technical designs
[Technical index](../technical_design/00_INDEX.md), [decisions](../technical_design/27_ARCHITECTURE_DECISIONS.md), [traceability](../technical_design/29_REQUIREMENTS_TRACEABILITY.md). Every workstream below links its governing design.

## 5. Prerequisites
Approved scope, decisions and bounded task packet; root [AGENTS](../../AGENTS.md) governs future agents.

## 6. Required decisions
Resolve release/root/identity before dependent implementation; approval, format, outcome and hosting policies remain pending. Do not substitute agent guesses.

## 7. Dependencies
Read strategy → decisions → slices → backlog → dependency matrix → relevant workstream → DoD/release. Contracts precede corresponding handlers even though contract governance is Phase 4.

## 8. Document catalogue

| Document | Workstream / purpose |
|---|---|
| [01 Strategy](01_IMPLEMENTATION_STRATEGY.md) | Phases, external-first scope and authority |
| [02 Standards](02_ENGINEERING_STANDARDS.md) | Cross-cutting ownership, review, types and changes |
| [03 Environment](03_DEVELOPMENT_ENVIRONMENT.md) | Phase 1 reproducible local setup plan |
| [04 Foundation](04_FOUNDATION_IMPLEMENTATION.md) | Phase 1 smallest real web/API smoke slice |
| [05 Database](05_DATABASE_IMPLEMENTATION.md) | Phase 2 constraints, migrations, sqlc and transactions |
| [06 Backend](06_BACKEND_IMPLEMENTATION.md) | Phase 3 domain commands and adapters |
| [07 Contracts](07_BACKEND_CONTRACTS_IMPLEMENTATION.md) | Phase 4 governance; per-operation approval happens first |
| [08 Authentication/authorization](08_AUTHENTICATION_AUTHORIZATION.md) | Phase 3 early identity prerequisite |
| [09 Provisioning](09_TENANT_PROVISIONING.md) | Phases 2–3 resumable isolated workspace |
| [10 Frontend](10_FRONTEND_IMPLEMENTATION.md) | Phase 5 usable real-contract screens |
| [11 Documents](11_DOCUMENT_STORAGE_IMPLEMENTATION.md) | Phase 6 quarantine/version/package integrity |
| [12 Events/workers](12_EVENTS_WORKERS_IMPLEMENTATION.md) | Phases 3/6 durable asynchronous effects |
| [13 Integrations](13_INTEGRATIONS_IMPLEMENTATION.md) | Phase 6 provider contracts and recovery |
| [14 AI/search](14_AI_SEARCH_IMPLEMENTATION.md) | Phase 7 later grounded assistance |
| [15 Security](15_SECURITY_IMPLEMENTATION.md) | Continuous controls; Phase 8 independent hardening |
| [16 QA](16_QA_TEST_IMPLEMENTATION.md) | Continuous tests; Phase 8 regression and UAT |
| [17 DevOps](17_DEVOPS_DEPLOYMENT.md) | Phases 1/9 reproducible artifact and deployment |
| [18 Observability](18_OBSERVABILITY_IMPLEMENTATION.md) | Continuous audit, signals and incident readiness |
| [19 Release](19_RELEASE_READINESS.md) | Phase 9 acceptance and operational gate |
| [20 Slices](20_VERTICAL_SLICES_ROADMAP.md) | Independently verifiable user outcomes |
| [21 Backlog](21_IMPLEMENTATION_BACKLOG.md) | Ordered BID-T000–017 tasks and evidence |
| [22 Definition of done](22_DEFINITION_OF_DONE.md) | Truthful completion states and checklist |
| [23 Dependencies](23_DEPENDENCY_MATRIX.md) | Prerequisite graph and safe parallelism |

## 9. Sequence
Approve alignment; establish foundation; implement identity/tenant boundary; ship manual opportunity and clean documents; matrix/response/review/package/proof/outcome; prove pilot reliability. Each operation crosses DB/API/UI/testing as needed.

## 10. Domain/security
Human decisions, isolated tenant data, explicit policy, immutable evidence, transaction/audit/outbox and retry safety are non-negotiable.

## 11. Database/API/frontend
Target paths are future artifacts, not files created in this documentation task. `<frontend-root>` remains pending path choice; no duplicate app.

## 12. Tests
Follow governing QA design and workstream; each task reports exact checks/results and limitations.

## 13. Acceptance
Documented user outcome runs against real dependencies with negative cases and traceability.

## 14. Done
Use source completion states and [DoD](22_DEFINITION_OF_DONE.md), not generic complete labels.

## 15. Risks
Read [technical risks](../technical_design/28_TECHNICAL_RISKS.md); unresolved policies block affected commands.

## 16. Verification evidence
Current documentation-only results appear in [quality report](../technical_design/QUALITY_REPORT.md). Application tests have not been executed because no implementation was created.

## 17. Exclusions
No application scaffolding, dependency installation, executable migrations or deployment in this task; full future schemas/contracts remain release-specific follow-up.
