# TenderOS — Product Delivery Plan: Waves and Sprints

## 1. Delivery model

- Default sprint length: **2 weeks**
- A wave contains one or more sprints and ends with an integrated, demonstrable operational outcome.
- Sequence is dependency-driven, not date-driven.
- Within every module/wave, delivery follows:

```text
Business operation and contract
→ Database/migrations/seed/reference data
→ Backend domain/application/API/security/events
→ Frontend routes/pages/forms/navigation/states
→ Integration/QA/security/performance/UAT evidence
```

The plan contains 17 waves and 33 indicative sprints. Calendar duration depends on team size, parallel work, template quality, infrastructure choice, and pilot feedback. A smaller team should preserve the sequence and reduce concurrent scope rather than skip quality gates.

## 2. Sprint operating rules

### Before a sprint

A ticket must have:

- Requirement IDs and persona outcome
- Dependencies
- Business rules and state transitions
- Data model/migration impact
- API/event contract
- Authorization matrix entry
- Page/route contract
- Acceptance tests and evidence

### During a sprint

- Database work merges before dependent backend completion.
- Backend contract and business logic pass integration tests before frontend is accepted.
- Frontend integrates real APIs; mocks are only temporary test fixtures.
- Reviewers assess product correctness, not only code style.
- Incomplete work is carried with explicit state; it is not marked done because a screen renders.

### Sprint review

Demonstrate the user operation in the target environment with real data and permissions. Capture evidence.

## 3. Wave overview

| Wave | Sprints | Outcome |
|---|---:|---|
| W00 | S00–S01 | Product, architecture, template, and delivery baseline |
| W01 | S02 | Repository, CI/CD, local platform, observability skeleton |
| W02 | S03–S04 | Control plane and private tenant database provisioning |
| W03 | S05–S06 | Authentication, memberships, context, and authorization |
| W04 | S07–S08 | Documents, audit, workflow, tasks, notifications, billing foundation |
| W05 | S09–S10 | Tenderpreneur onboarding and compliance vault |
| W06 | S11–S12 | External tender import and deterministic opportunity workspace |
| W07 | S13–S14 | Bid workspace, compliance, proposal, readiness, external submission evidence |
| W08 | S15–S16 | Deep Search baseline, saved searches, watchlists, and alerts |
| W09 | S17–S18 | AI platform, OCR/extraction, citations, Bid/No-Bid assistance |
| W10 | S19–S20 | Procurement planning, requisition, and tender authoring |
| W11 | S21–S22 | Review, approval, publication, catalogue, clarifications, addenda |
| W12 | S23–S24 | Platform submissions, sealing, receipts, withdrawal/replacement |
| W13 | S25–S26 | Bid opening and integrity evidence |
| W14 | S27–S29 | Preliminary, technical, financial evaluation and award |
| W15 | S30–S31 | Procurement AI, advanced Tenderpreneur AI, and intelligence |
| W16 | S32 | Internationalization, enterprise hardening, pilot and launch gate |

## 4. W00 — Product and design baseline

### S00 — Product model and requirements

**Database/design**

- Define control, tenant, exchange, search, and object-storage ownership.
- Define tenant types and country-pack model.

**Backend/API design**

- Define bounded contexts, state machines, API standards, error envelope, events, authorization vocabulary.

**Frontend/design**

- Define route groups, portal navigation, personas, page-contract template, visual principles.

**QA/evidence**

- Requirement IDs, traceability template, risk register, initial UAT scenarios.

**Exit:** BRD/PRD/SRD accepted.

### S01 — Architecture spikes and template audit

- Prove dynamic tenant database routing and bounded pool management.
- Prove outbox/inbox and exchange projection.
- Prototype sealed-envelope/key-release approach without production claims.
- Materialize/audit downloaded frontend template: license, dependencies, components, responsiveness, accessibility, design tokens.
- Select deployment, message, storage, identity, and payment adapters through ADRs.

**Exit:** High-risk architecture decisions have evidence or explicit follow-up.

## 5. W01 — Engineering platform

### S02 — Repository and environments

**Database**

- Local control, exchange, and sample tenant databases.
- Migration runners and schema-version tables.

**Backend**

- Go modules, API/worker skeleton, health, config, structured errors/logging.
- Python AI API/worker skeleton.

**Frontend**

- Next.js application, template-derived design tokens/primitives, public/auth shells.

**Operations/QA**

- Docker Compose, CI lint/test/build/scan, testcontainers, OpenTelemetry skeleton, seed data.

**Exit:** Clean checkout starts and tests all deployables with one documented workflow.

## 6. W02 — Control plane and tenant provisioning

### S03 — Control database and Platform Admin tenant registry

**Database first**

- Control schemas: tenant registry, clusters, provisioning jobs, plans, feature flags, platform audit.
- Seed tenant types and initial country pack.

**Backend**

- Tenant registry/admin APIs, provisioning command model, cluster selection, audit.

**Frontend**

- Platform Admin shell, tenant list/detail, provisioning status, platform settings baseline.

**QA**

- Admin authorization, error states, schema/migration tests.

### S04 — Private database provisioning and routing

**Database**

- Common tenant schema baseline and database roles.
- Exchange schema baseline.

**Backend**

- Provision database/roles/secrets/storage, migrate/seed, activate/suspend.
- Tenant DB router, pool limits/eviction, health, schema compatibility.

**Frontend**

- Onboarding provisioning progress, failure/retry/support path.

**QA**

- Multi-tenant create/read/write isolation, failed provisioning resume, restore-to-new proof.

**Wave exit:** Two Procurement and two Tenderpreneur tenants provision and remain isolated.

## 7. W03 — Identity, memberships, context, and authorization

### S05 — Authentication and session lifecycle

**Database**

- Identities, emails, credentials, sessions, recovery, MFA state.

**Backend**

- Register, verify, login, logout, recovery, rotation, revocation, stable 401 semantics.

**Frontend**

- Auth routes/forms, verification, recovery, session-expiry handling.

**QA**

- Enumeration resistance, rotation, revocation, rate limiting, browser security.

### S06 — Memberships, context, roles, capabilities

**Database**

- Memberships, invitations, role grants, tenant user projection.

**Backend**

- `/v1/me`, membership enumeration, context selection, effective capabilities, 403 semantics, authorization middleware.

**Frontend**

- Workspace switcher, invitation acceptance, team/role pages, route/nav filtering.

**QA**

- Full role/context matrix, cross-tenant IDs, cache clearing, last-admin rule.

**Wave exit:** Authenticated context is production-ready for downstream modules.

## 8. W04 — Shared operating foundation

### S07 — Documents, audit, and eventing

**Database**

- Documents/versions/files, audit, outbox/inbox, idempotency.

**Backend**

- Upload lifecycle, object storage adapter, scan state, download authorization, audit and event dispatch.

**Frontend**

- File uploader, document list/detail/versions, processing/quarantine/error states, activity timeline.

**QA**

- Malware fixtures, type/size limits, signed URLs, object isolation, event replay.

### S08 — Workflows, tasks, notifications, subscription foundation

**Database**

- Workflow templates/instances, approvals, tasks/dependencies, notification outbox, plans/subscriptions/usage.

**Backend**

- Serial/parallel approvals, task operations, notifications, entitlement checks, usage ledger.

**Frontend**

- Pending approvals, task boards/lists, comments, notifications, plan/usage pages.

**QA**

- Delegation/rework/expiry, notification retry, quota race/idempotency.

**Wave exit:** All later modules can reuse governed documents, approvals, tasks, audit, and entitlements.

## 9. W05 — Tenderpreneur onboarding and compliance vault

### S09 — Business profile and capability records

**Database**

- Products/services, sectors, regions, certifications, staff, projects, references.

**Backend**

- CRUD through domain use cases, validation, classifications, permissions.

**Frontend**

- Onboarding wizard and profile pages with correct structured controls.

**QA**

- Permission, validation, completeness, responsive/accessibility tests.

### S10 — Compliance vault and expiry readiness

**Database**

- Document categories, verification, expiry rules, knowledge versions/approvals, usage links.

**Backend**

- Vault operations, version/verification/expiry, reusable knowledge approval, readiness query.

**Frontend**

- Vault list/detail/upload, expiry calendar/alerts, knowledge approval.

**QA**

- Expired/superseded evidence, financial-document restriction, AI-access eligibility.

**Wave exit:** Tenderpreneur maintains trustworthy reusable evidence.

## 10. W06 — External tender import and deterministic workspace

### S11 — Import pipeline without AI dependency

**Database**

- Opportunities, source/provenance, imports, fact fields, verification.

**Backend**

- File, URL, text, manual import; safe fetch; duplicate candidate; basic metadata capture.

**Frontend**

- Import wizard, upload/fetch progress, manual facts, duplicate resolution.

**QA**

- SSRF, inaccessible URLs, encrypted/corrupt files, duplicate and provenance tests.

### S12 — Opportunity verification and workspace creation

**Database**

- Opportunity version, tender snapshot, workspace linkage.

**Backend**

- Fact verification, create workspace, deadlines/source changes, activity.

**Frontend**

- Opportunity detail, verify facts, source viewer, create/assign workspace.

**QA**

- Low-confidence/manual corrections and immutable source/version links.

**Wave exit:** Any tender can become a governed workspace even with AI disabled.

## 11. W07 — Bid execution MVP

### S13 — Bid/No-Bid, requirements, tasks, and evidence

**Database**

- Bid assessments/decisions, requirements, assignments, evidence, risks.

**Backend**

- Weighted assessment, human decision, requirement transitions, evidence validation, dependencies.

**Frontend**

- Bid/No-Bid page, compliance matrix, side-panel detail, bulk assignment, task views.

**QA**

- Mandatory gate, evidence version, role and concurrency tests.

### S14 — Proposal, pricing, approvals, readiness, external evidence

**Database**

- Response versions, financial lines, reviews, readiness, packages, external evidence/outcomes.

**Backend**

- Technical/financial response, restricted pricing, approvals, deterministic readiness, package manifest, submitted-evidence rule.

**Frontend**

- Response editor, pricing workspace, review/approval, readiness and submission pages, evidence capture.

**QA/UAT**

- Full external tender journey; no mock data; version invalidation and access controls.

**Wave exit / MVP-1:** A Tenderpreneur can execute an external tender end to end.

## 12. W08 — Deep Search baseline

### S15 — Source registry connectors and public search index

**Database**

- Source documents/fetches/canonical opportunities/source links/change events.

**Backend/search workers**

- Initial approved connectors, scheduler, conditional fetch, parse, normalize, deduplicate, index.

**Frontend**

- Discover page, filters, source/freshness display, original source.

**QA**

- Connector fixtures, rate limit, parsing, authoritative-source and dedup tests.

### S16 — Matching, saved searches, watchlists, and alerts

**Database**

- Search preferences, saved searches, watch targets, result/match factors, alerts.

**Backend**

- Hybrid search baseline, explainable match factors, saved execution, change alerts, workspace conversion.

**Frontend**

- Natural-language search, saved searches, watchlists, match/gap explanation.

**QA/UAT**

- Relevance benchmark, source freshness, alert correctness, search-to-workspace journey.

**Wave exit / MVP-2:** Tenderpreneurs discover and import traceable opportunities.

## 13. W09 — AI foundation and Tenderpreneur analysis

### S17 — AI jobs, ingestion, OCR, retrieval, citations

**Database**

- AI jobs/inputs/outputs/citations/usage, chunks/embeddings, prompt/model registry.

**Backend/AI**

- Job authorization, provider adapters, extraction/OCR, chunking, retrieval, structured output/citation validation.

**Frontend**

- AI job progress, result/source viewer, warnings, accept/edit/reject.

**QA**

- Benchmark dataset, prompt-injection, cross-tenant retrieval, provider outage, cost metrics.

### S18 — Tender extraction, Bid/No-Bid and compliance assistance

**Backend/AI**

- Structured tender extraction, requirement proposals, fit and evidence matching, addendum impact.

**Frontend**

- Verify extracted facts/requirements, AI assessment, suggested evidence, source navigation.

**QA/UAT**

- Mandatory recall, citation correctness, unsupported-claim, human review behavior.

**Wave exit:** AI accelerates but does not own the external tender workflow.

## 14. W10 — Procurement planning and authoring

### S19 — Plans, requests, budgets, methods, approvals

**Database**

- Procurement plans/items, requests/versions, budget confirmations, method selections.

**Backend**

- Request lifecycle, approval, conversion.

**Frontend**

- Plan/request lists, forms, approval pages, actionable dashboard queues.

**QA/UAT**

- Requester-to-approved-request journey and separation of duty.

### S20 — Tender draft, lots, requirements, criteria, schema

**Database**

- Tender/versions/lots/schedules/requirements/criteria/forms/submission fields.

**Backend**

- Domain invariants, validation, versioning, review readiness.

**Frontend**

- Tender authoring workspace with all sections, correct controls, review findings.

**QA**

- Date/weight/threshold and version/concurrency tests.

**Wave exit:** Procurement Team produces a valid tender candidate.

## 15. W11 — Approval, publication, clarification, and addenda

### S21 — Tender review, approval, and publication

**Database**

- Review findings, publication intents/deliveries, public exchange projections.

**Backend**

- Version-specific review/approval, publish outbox, projection/hash reconciliation, catalogue APIs.

**Frontend**

- Review/approval workspace, publication settings, public tender page.

**QA**

- Approval invalidation, publication failure/retry, public/private data leakage tests.

### S22 — Clarifications, addenda, invitations, and closing

**Database**

- Questions/responses/addenda/change records/acknowledgements/invitations.

**Backend**

- Clarification privacy, addendum version, impact events, deadline extension, auto-close.

**Frontend**

- Procurement clarification workspace, Tenderpreneur/public questions, addendum acknowledgement.

**QA/UAT**

- Material addendum changes and readiness invalidation.

**Wave exit / MVP-3:** Procurement Teams can publish and manage live tenders.

## 16. W12 — Direct submissions and sealed exchange

### S23 — Submission schema, final validation, encryption, receipt

**Database**

- Exchange submissions/versions/envelopes/manifests/receipts.

**Backend**

- Final validation, lock response, encrypt/seal, server timestamp, acknowledgement, metadata event.

**Frontend**

- Platform submission page, declaration, step-up, progress, receipt.

**QA/security**

- Idempotency, deadline boundary, content hash, tamper, no pre-opening content.

### S24 — Withdrawal, replacement, closing spike, incident controls

**Database/backend**

- Lineage, withdrawal/replacement policy, reconciliation, deadline reservation if needed.

**Frontend**

- Replace/withdraw with consequences and history.

**QA/performance**

- Concurrent closing spike, retries, interruption, queue/storage/KMS failure, evidence.

**Wave exit:** Sealed submission is independently security-reviewed for pilot.

## 17. W13 — Bid opening

### S25 — Committee, declarations, quorum, and release

**Database**

- Opening session/members/declarations/release records.

**Backend**

- Time/state/quorum/conflict/step-up/key-release policies.

**Frontend**

- Opening preparation and controlled start/release experience.

**QA/security**

- Invalid opening attempts, partial failure, double release, conflict cases.

### S26 — Integrity verification and minutes

**Database/backend**

- Manifest verification, opening facts, minutes, closure/evidence package.

**Frontend**

- Submission list after release, integrity states, minutes and sign-off.

**QA/UAT**

- Full submission-to-opening pilot scenario.

## 18. W14 — Evaluation and award

### S27 — Preliminary evaluation

- Database findings/evidence/clarifications.
- Backend pass/fail rules, disqualification evidence, stage progression.
- Frontend compliance matrix and submission viewer.
- QA on mandatory criteria, permissions, and audit.

### S28 — Technical and financial evaluation

- Database assignments/scores/arithmetic corrections/locks.
- Backend independent scoring, stage restrictions, consolidation.
- Frontend criterion scoring split view, restricted financial workspace.
- QA on unpublished criteria, score ranges, conflict and locking.

### S29 — Moderation, recommendation, award, notices, handover

- Database results/reports/recommendations/award/notices/handover.
- Backend approvals, outcome events, report export.
- Frontend panel results, recommendation/award workflow, Tenderpreneur outcome.
- UAT complete platform tender lifecycle.

**Wave exit / MVP-4:** Closed-loop procurement lifecycle works end to end.

## 19. W15 — Advanced AI and intelligence

### S30 — Tenderpreneur advanced AI

- Proposal drafting with verified knowledge
- Contradiction/unsupported claim review
- Submission audit
- Win/loss learning
- AI benchmark and cost gates

### S31 — Procurement AI and analytics

- Tender draft/quality review
- Criteria assistance
- Evidence locator/bid comparison
- Price/outlier support
- Draft evaluation reports
- Procurement, Tenderpreneur, and platform analytics/read models

**Exit:** Human authority is preserved; benchmarks and pilot value pass.

## 20. W16 — Internationalization, hardening, pilot, launch

### S32 — Release hardening

**Database/operations**

- Migration fleet rehearsal, backup/restore, retention/legal hold, scale tests.

**Backend/security**

- Country-pack validation, SSO/API readiness as scoped, VAPT remediation, threat model update.

**Frontend**

- Localization, accessibility remediation, browser/device coverage, help/onboarding content.

**QA/UAT**

- Full regression, performance, DR exercise, pilot sign-off, release evidence.

**Exit:** Production launch gate passed.

## 21. Ticket hierarchy

```text
Release
  → Wave
    → Epic/Module
      → User journey slice
        → Database ticket
        → Backend/API ticket
        → Frontend ticket
        → QA/evidence ticket
```

Do not create frontend-only “complete page” tickets without their operational contracts and dependencies.

## 22. Developer execution packet

Every implementation packet should include:

- Ticket ID/title and requirement IDs
- User/business outcome
- Current code/spec evidence
- Dependencies and blocked-by list
- Domain model and rules
- Migration/schema changes and seed data
- API/event changes and examples
- Authorization matrix
- UI route/page/form/states
- Test cases and evidence path
- Non-goals
- Rollout/feature flag/migration plan
- Completion checklist

## 23. Release branch and review gates

- Trunk-based or short-lived branches
- Mandatory reviews for domain, migration, security, API, and UI changes
- Contract-breaking changes blocked by CI unless versioned/approved
- Feature flags for incomplete or risky functionality
- No production flag activation before UAT/security/operations evidence
- Release notes map to requirement and evidence IDs

