# TenderOS — Module Catalogue and Dependency Order

## 1. Purpose

This document prevents downstream modules from being built on missing foundations. A module may be designed early, but production implementation must follow its dependency gates.

## 2. Dependency graph

```text
M00 Product Governance & Contracts
  └─ M01 Repository, CI/CD & Environments
      ├─ M02 Platform Control Plane
      │   ├─ M03 Tenant Provisioning & Database Routing
      │   │   └─ M04 Identity, Membership, Context & Authorization
      │   │       ├─ M05 Subscription, Entitlements & Usage
      │   │       ├─ M06 Audit & Evidence
      │   │       ├─ M07 Document & Object Storage
      │   │       ├─ M08 Workflow, Approval, Tasks & Notifications
      │   │       └─ M09 Search & Read-Model Foundation
      │   │
      │   ├─ Procurement branch
      │   │   └─ M10 Procurement Planning & Requisitions
      │   │       └─ M11 Tender Authoring, Criteria & Review
      │   │           └─ M12 Publication, Catalogue, Clarifications & Addenda
      │   │               └─ M17 Submission Exchange & Bid Sealing
      │   │                   └─ M18 Bid Opening
      │   │                       └─ M19 Evaluation
      │   │                           └─ M20 Award & Handover
      │   │
      │   └─ Tenderpreneur branch
      │       └─ M13 Organization Profile & Compliance Vault
      │           ├─ M14 External Tender Import & Ingestion
      │           ├─ M15 Deep Search, Matching & Watchlists
      │           └─ M16 Bid Workspace, Compliance, Proposal & Readiness
      │               └─ M17 Submission Exchange & Bid Sealing
      │
      └─ Intelligence branch
          └─ M21 AI Platform & Evaluation Foundation
              ├─ M22 Tenderpreneur AI
              ├─ M23 Procurement AI
              └─ M24 Analytics, Intelligence & Reporting

M25 Internationalization, Security Hardening & Enterprise Integrations
spans and validates all modules; it is introduced early and completed progressively.
```

## 3. Module specifications

## M00 — Product governance and contracts

**Purpose:** Establish requirements, terminology, business rules, traceability, OpenAPI/event conventions, decision process, and acceptance gates.

**Outputs:** Approved specs, requirement IDs, decision log, contract templates, evidence register.

**Exit gate:** No ambiguous portal or tenancy model; MVP scope and precedence accepted.

## M01 — Repository, CI/CD, environments, and developer platform

**Dependencies:** M00

**Outputs:** Monorepo, lint/test/build pipelines, local Docker Compose, secret handling, code owners, branch protections, image builds, testcontainers, observability skeleton.

**Exit gate:** One command starts local dependencies; sample API/web/worker/AI health checks pass in CI.

## M02 — Platform control plane

**Dependencies:** M01

**Owns:** Tenant registry, plans, country packs, source registry, feature flags, platform users/roles, provisioning jobs, service metadata.

**Exit gate:** Platform Admin can view/control non-sensitive platform metadata with full audit.

## M03 — Tenant provisioning and database routing

**Dependencies:** M02

**Owns:** Database allocation, credentials, schema version, migration orchestration, storage namespace, tenant lifecycle, connection routing.

**Exit gate:** Automated create, migrate, suspend, restore-to-new, and decommission tests pass for multiple tenants.

## M04 — Identity, membership, context, and authorization

**Dependencies:** M02, M03

**Owns:** Identity, sessions, invitations, memberships, roles, policies, active context, MFA/step-up foundation.

**Exit gate:** `/v1/me`, context selection, capability evaluation, revocation, 401/403 semantics, and cross-tenant tests pass.

## M05 — Subscription, entitlements, usage, and billing

**Dependencies:** M02, M04

**Owns:** Plans, subscriptions, limits, usage meters, reservations, invoices, payment/webhook state.

**Exit gate:** Backend blocks over-limit operations consistently and handles webhook replay idempotently.

## M06 — Audit and evidence

**Dependencies:** M03, M04

**Owns:** Append-only audit, correlation, evidence manifests, exports, support-access logs.

**Exit gate:** Material operations produce complete, queryable evidence without sensitive diagnostic leakage.

## M07 — Document and object storage

**Dependencies:** M03, M04, M06

**Owns:** Upload, scan, checksum, metadata, version, access, expiry, preview, retention, signed download.

**Exit gate:** Tenant isolation, malware quarantine, version immutability, large upload, and signed URL tests pass.

## M08 — Workflow, approval, tasks, and notifications

**Dependencies:** M04, M05, M06, M07

**Owns:** Workflow templates/instances, approvals, task dependencies, comments, notification preferences and dispatch.

**Exit gate:** Serial/parallel approval, rework, delegation, expiry, idempotent notification, and permission tests pass.

## M09 — Search and read-model foundation

**Dependencies:** M03, M04, M06

**Owns:** Tenant-private search contracts, public catalogue index contracts, rebuild/version/freshness, reporting projections.

**Exit gate:** Search never reveals unauthorized record existence; indexes rebuild from source data.

## M10 — Procurement planning and requisitions

**Dependencies:** M04–M08

**Owns:** Plans, requests, budgets, methods, approvals, conversion.

**Exit gate:** Requester-to-approved-requisition journey passes UAT.

## M11 — Tender authoring, criteria, and review

**Dependencies:** M10, M07, M08

**Owns:** Tender drafts, lots, schedule, requirements, criteria, forms, submission schema, review/approval.

**Exit gate:** An approved, internally consistent version can be produced; invalid criteria/dates cannot pass.

## M12 — Publication, catalogue, clarifications, and addenda

**Dependencies:** M09, M11

**Owns:** Publication version, public projection, invitation, question/response, addendum, deadline changes.

**Exit gate:** Public page matches approved hash/version; addendum impact and notifications work end to end.

## M13 — Tenderpreneur profile and compliance vault

**Dependencies:** M04–M08

**Owns:** Company capabilities, document types, staff, certifications, projects, references, knowledge approval.

**Exit gate:** Authorized users can maintain reusable, versioned, expiry-aware evidence.

## M14 — External tender import and ingestion

**Dependencies:** M07, M09, M13; AI extraction may be added after M21

**Owns:** File/URL/email/text/manual import, provenance, basic deterministic metadata capture, duplicate detection, verification.

**Exit gate:** An external tender workspace can be created without AI and remains fully usable.

## M15 — Deep Search, matching, and watchlists

**Dependencies:** M02 source registry, M09, M13; advanced semantic ranking depends on M21

**Owns:** Source fetch, canonicalization, deduplication, search filters, match factors, watchlists, alerts, workspace conversion.

**Exit gate:** Approved sources produce fresh, traceable, deduplicated results with one-click import.

## M16 — Bid workspace, compliance, proposal, and readiness

**Dependencies:** M08, M13, M14 and/or M12

**Owns:** Bid decision, requirements, tasks, response content, pricing, reviews, approvals, readiness, package/evidence.

**Exit gate:** External tender journey completes end to end without mocks.

## M17 — Submission exchange and bid sealing

**Dependencies:** M11, M12, M16, M06, M07

**Owns:** Submission schema validation, final version, envelope, encryption/sealing, receipt, withdrawal, replacement, opening release.

**Exit gate:** Procurement users cannot access content before authorized release; spike and replay tests pass.

## M18 — Bid opening

**Dependencies:** M17, M08

**Owns:** Committee, quorum, declarations, release, integrity verification, minutes.

**Exit gate:** Authorized opening succeeds and all invalid opening attempts fail closed with evidence.

## M19 — Evaluation

**Dependencies:** M18, M11, M08

**Owns:** Preliminary, technical, financial, evaluator assignment, score lock, moderation, clarification, report.

**Exit gate:** Published criteria drive evidence-linked, permission-scoped evaluation through consolidated result.

## M20 — Award and handover

**Dependencies:** M19, M08

**Owns:** Recommendation, approval, outcome notices, award publication, contract handover.

**Exit gate:** Award journey completes with approved notices and immutable evidence.

## M21 — AI platform and evaluation foundation

**Dependencies:** M03, M04, M06, M07, M09

**Owns:** AI jobs, provider adapters, retrieval authorization, structured outputs, citations, prompt/model registry, cost/usage, evaluation datasets.

**Exit gate:** A grounded extraction job is reproducible, tenant-safe, measured, and evaluated.

## M22 — Tenderpreneur AI

**Dependencies:** M13–M16, M21

**Owns:** Tender extraction, Bid/No-Bid assistance, compliance matching, proposal drafting, submission audit.

**Exit gate:** Offline benchmark and UAT meet agreed citation/accuracy thresholds; no high-risk automatic transition.

## M23 — Procurement AI

**Dependencies:** M10–M12, M18–M21

**Owns:** Tender drafting/review, criteria assistance, evidence location, bid comparison, price analysis, report drafting.

**Exit gate:** Evaluators can inspect sources and accept/edit/reject suggestions; final scoring remains manual.

## M24 — Analytics, intelligence, and reporting

**Dependencies:** M06, M09 and relevant domain modules

**Owns:** Tenant dashboards, platform metrics, pipeline, cycle time, win/loss, disqualification, participation, usage, AI quality.

**Exit gate:** Every metric has a defined source, formula, freshness, permission, and reconciliation test.

## M25 — Internationalization, security hardening, and enterprise integrations

**Dependencies:** Cross-cutting

**Owns:** Country packs, translations, SSO, APIs/webhooks, retention, legal hold, advanced security, dedicated infrastructure, data export/import.

**Exit gate:** Country/enterprise release checklist passes without modifying core domain invariants.

## 4. Dependency enforcement rules

1. A downstream frontend page cannot be accepted before its backend contract and domain rules exist.
2. A backend use case cannot be accepted before database constraints/migrations and authorization policy exist.
3. AI may enhance M14–M20 only after deterministic workflows work without AI.
4. Analytics may not become a substitute for operational capabilities.
5. Cross-tenant exchange must use M17; direct tenant database reads are prohibited.
6. Each module must publish explicit APIs/events rather than allowing table access from another module.
7. Every wave must include migration, API, UI, tests, evidence, and operational runbook changes where relevant.

