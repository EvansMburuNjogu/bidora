# Requirements traceability

Status: design coverage, not implementation acceptance. Sources are the existing product documents below. IDs beginning BID-T are new task IDs in [backlog](../implementation/21_IMPLEMENTATION_BACKLOG.md), not source requirements. Verification refers to future required evidence, not executed application tests.

## Source-to-design-to-task matrix

| Product source / actual requirement or section | Technical design authority | Tasks / workstream | Required verification |
|---|---|---|---|
| [00 master](../product/00_MASTER_INDEX.md): precedence and completion | [Decisions](27_ARCHITECTURE_DECISIONS.md), [system](01_SYSTEM_ARCHITECTURE.md) | T000; strategy/DoD | Approved conflict/release mapping; evidence states truthful |
| [01 definition](../product/01_PRODUCT_DEFINITION.md): external/closed operating models | [System](01_SYSTEM_ARCHITECTURE.md), [domain](03_DOMAIN_ARCHITECTURE.md) | T000/T007–015 | External journey independent of buyer marketplace; no fake platform completion |
| [02 BRD](../product/02_BRD.md): BR-001–006 | [Tenancy](04_MULTI_TENANCY_DESIGN.md), [identity](11_AUTHENTICATION_AUTHORIZATION.md) | T003–005 | Private DB, explicit context, quotas, suspension and support denial |
| [02 BRD](../product/02_BRD.md): BR-020–026, BR-050/051 | [Schema](06_DATABASE_SCHEMA_DESIGN.md), [modules](09_BACKEND_MODULE_DESIGN.md), [audit](20_AUDIT_LOGGING_EVIDENCE.md) | T007–014 | Exact source/evidence/approved package; pricing isolation; immutable history |
| [03 PRD](../product/03_PRD.md): PR-TP-004/006–014 | [API](10_API_CONTRACT_DESIGN.md), [screens](12_FRONTEND_ARCHITECTURE.md) | T007–014; slices S3–S9 | Capture, decision, matrix, response, review, package, proof and outcome E2E |
| [03 PRD](../product/03_PRD.md): PR-SH-001–009, UX-001–008 | [Identity](11_AUTHENTICATION_AUTHORIZATION.md), [components](13_FRONTEND_COMPONENT_DESIGN.md), [state](14_STATE_MANAGEMENT_DATA_FLOW.md) | T003–006/T008–015 | Auth, version conflict, error states, accessible keyboard/mobile and real APIs |
| [04 SRD](../product/04_SRD.md): SR-DEP, API-001–008, DATA-001–008 | [Backend](08_BACKEND_ARCHITECTURE.md), [API](10_API_CONTRACT_DESIGN.md), [DB](05_DATABASE_ARCHITECTURE.md) | T001/T002 and contract workstream | Type/contract compatibility, transactions, parameterized SQL, safe DTOs |
| [04 SRD](../product/04_SRD.md): SR-TEN, SR-IAM | [Tenancy](04_MULTI_TENANCY_DESIGN.md), [identity](11_AUTHENTICATION_AUTHORIZATION.md) | T003–005 | Provisioning checkpoints, revoked membership, schema/pool isolation |
| [05 domain](../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md): RULE-BID/DOC/IAM | [Domain](03_DOMAIN_ARCHITECTURE.md), [schema](06_DATABASE_SCHEMA_DESIGN.md) | T004/T008–013 | Invalid transitions, expiry, current candidate, last-admin and concurrency tests |
| [05 domain](../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md): RULE-SUB/OPEN/EVAL/AWD | [Security](19_SECURITY_ARCHITECTURE.md), [decisions](27_ARCHITECTURE_DECISIONS.md) | T017 future | Complete future schemas/cards and independent release review; not covered as implemented MVP |
| [06 journeys](../product/06_USER_JOURNEYS_AND_WORKFLOWS.md): J-01/J-06–J-10/J-16 | [Screens](12_FRONTEND_ARCHITECTURE.md), [audit](20_AUDIT_LOGGING_EVIDENCE.md) | T003–014 | Multi-role external journey, evidence confirmation, role/context switch |
| [07 dependencies](../product/07_MODULE_DEPENDENCIES.md): M00–M08/M13/M14/M16 | [Modules](09_BACKEND_MODULE_DESIGN.md), [repository](02_REPOSITORY_ARCHITECTURE.md) | T000–015; dependency matrix | No auth/admin inversion; no direct cross-module persistence |
| [08 HLD](../product/08_HLD.md): planes, outbox, sealed flow | [System](01_SYSTEM_ARCHITECTURE.md), [events](17_EVENT_MESSAGING_DESIGN.md) | T005/T006/T017 | No cross-plane SQL; replay and future acceptance reconciliation |
| [09 LLD](../product/09_LLD.md): transactions/errors/idempotency | [Backend](08_BACKEND_ARCHITECTURE.md), [API](10_API_CONTRACT_DESIGN.md) | T002/T004/T006 and each command | Atomic audit/outbox; stale/repeated command behavior |
| [10 database](../product/10_DATABASE_DESIGN.md): ownership/constraints/migrations | [DB](05_DATABASE_ARCHITECTURE.md), [schema](06_DATABASE_SCHEMA_DESIGN.md), [migrations](07_DATABASE_MIGRATIONS.md) | T002/T005/T008–014 | Clean/upgrade migrations, FKs/checks, immutable rows, restore |
| [11 API](../product/11_BACKEND_API_DESIGN.md): routes/security/contracts | [API](10_API_CONTRACT_DESIGN.md), [integration](16_INTEGRATION_ARCHITECTURE.md) | Each task contract review | OpenAPI examples, safe errors, webhook replay, generated-client drift |
| [12 frontend](../product/12_FRONTEND_UX_DESIGN.md): routes/components/state | [Frontend](12_FRONTEND_ARCHITECTURE.md), [components](13_FRONTEND_COMPONENT_DESIGN.md), [state](14_STATE_MANAGEMENT_DATA_FLOW.md) | T001/T003–015 | No private cache leakage; accessible forms/matrix and honest async states |
| [13 AI/search](../product/13_AI_DEEP_SEARCH_SPEC.md): citation/safety/evaluation | [AI/search](18_AI_SEARCH_ARCHITECTURE.md) | T016 later | Authorized retrieval, corpus metrics, prompt injection, no autonomous decisions |
| [14 security](../product/14_SECURITY_PRIVACY_COMPLIANCE.md): threat model and gates | [Security](19_SECURITY_ARCHITECTURE.md), [storage](15_DOCUMENT_STORAGE_DESIGN.md) | All tasks, T015/T017 independent gates | Negative policy/file/SSRF/CSRF tests, scans and privacy review |
| [15 QA](../product/15_QA_TEST_STRATEGY.md): test levels/quality gates | [QA](24_QA_TEST_ARCHITECTURE.md) | All tasks, test workstream | Reports with actual executions, failures/skips and scope |
| [16 roadmap](../product/16_PRODUCT_ROADMAP.md): R0/R1/R2 and later gates | [Decisions](27_ARCHITECTURE_DECISIONS.md), [risks](28_TECHNICAL_RISKS.md) | T000/T015–017 | Approved external release boundary separate from full product |
| [17 delivery](../product/17_DELIVERY_PLAN_WAVES_SPRINTS.md): W00–W07, S11–S14 | [Modules](09_BACKEND_MODULE_DESIGN.md) | T000–015; slices roadmap | Real operations database→API→UI→QA, no frontend-only completion |
| [18 operations](../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md): environments/CI/DR/SLO | [Deployment](21_DEPLOYMENT_ARCHITECTURE.md), [CI](22_DEVOPS_CICD_DESIGN.md), [operations](23_OBSERVABILITY_OPERATIONS.md), [performance](25_PERFORMANCE_SCALABILITY.md), [DR](26_DISASTER_RECOVERY.md) | T001/T005/T006/T015 | Immutable promotion, bounded fleets, measured load, alert and restore drills |
| [19 traceability](../product/19_TRACEABILITY_ACCEPTANCE_EVIDENCE.md): completion/sign-off | This matrix, [QA](24_QA_TEST_ARCHITECTURE.md) | All tasks; DoD/release workstreams | Requirement-build-test-UAT chain, no invented evidence |
| [20 template](../product/20_FRONTEND_TEMPLATE_ADOPTION.md): adoption stages | [Components](13_FRONTEND_COMPONENT_DESIGN.md) | T001 and frontend workstream | License/script/asset audit or explicit product-owned baseline decision |
| [21 technology](../product/21_TECHNOLOGY_DECISIONS.md): stack/ports | [Decisions](27_ARCHITECTURE_DECISIONS.md), [integrations](16_INTEGRATION_ARCHITECTURE.md) | T000/T001/T016 | Version compatibility, provider abstraction and dependency review |
| [22 glossary/decisions](../product/22_GLOSSARY_ASSUMPTIONS_DECISIONS.md): open policies | [Register](27_ARCHITECTURE_DECISIONS.md), [risks](28_TECHNICAL_RISKS.md) | T000 and dependent gates | Owners, status, approvals and legacy-qualified aliases |
| [23 agents](../product/23_AGENTS.md): execution rules | [Root AGENTS](../../AGENTS.md), [repository](02_REPOSITORY_ARCHITECTURE.md) | Every authorized task | Bounded change, source/contract review, actual test report |

## Coverage limits

MVP depth covers principal control/tenant tables, significant external workflow API cards, module commands and screen contracts. Full future procurement planning/moderation/award, billing/invoices, enterprise identity and AI data stores require release-specific detailed schemas and cards. Existing product catalogue remains source of scope; this matrix does not claim every future requirement fully designed or implemented.

## Evidence record contract

Future records include task/source IDs, design/decision IDs, build, environment, execution time, commands/tool versions, actual result, artifact paths, limitations and approver. Source examples showing PASS are examples, not executed evidence. Current documentation validation belongs in [quality report](QUALITY_REPORT.md).
