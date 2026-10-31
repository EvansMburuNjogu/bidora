# TenderOS — Requirements Traceability, Acceptance, and Evidence

## 1. Purpose

This document defines how business intent remains traceable through product requirements, system design, implementation, tests, UAT, and release decisions.

## 2. Traceability chain

```text
Business Objective
→ Business Requirement
→ Product Requirement
→ System Requirement / Business Rule
→ Module and Architecture Component
→ Database Migration / API / Event / Page
→ Automated Test / UAT Scenario
→ Evidence Artifact
→ Release and Sign-off
```

No requirement is considered delivered because code exists. It is delivered when the traceability chain and acceptance evidence are complete.

## 3. Identifier conventions

| Prefix | Meaning | Example |
|---|---|---|
| `BO` | Business objective | `BO-02` |
| `BR` | Business requirement | `BR-024` |
| `PR` | Product requirement | `PR-TP-011` |
| `SR` | System requirement | `SR-SUB-003` |
| `RULE` | Domain invariant | `RULE-SUB-003` |
| `DATA` | Data requirement | `DATA-006` |
| `API` | API requirement | `API-004` |
| `UX` | Experience requirement | `UX-005` |
| `SEC` | Security requirement | `SEC-007` |
| `AI` | AI requirement | `AI-EXTRACT-001` |
| `SEARCH` | Deep Search requirement | `SEARCH-DEDUP-001` |
| `QA` | Quality requirement | `QA-ISO-001` |
| `OPS` | Operational requirement | `OPS-003` |
| `ADR` | Architecture decision | `ADR-005` |
| `EVID` | Evidence item | `EVID-W12-004` |

## 4. Traceability matrix baseline

| Business | Product/System | Module | Primary implementation | Acceptance evidence |
|---|---|---|---|---|
| BR-001–006 | PR-SH-001/002/009; SR-TEN/IAM | M02–M05 | Control DB, tenant router, `/me`, context, entitlements | Provisioning, context, isolation, billing tests |
| BR-010 | PR-PT-001 | M10 | Plans/requests DB, APIs, pages | Requester-to-approval UAT |
| BR-011 | PR-PT-002 | M11 | Tender schema, authoring API/workspace | Valid/invalid tender test set |
| BR-012 | PR-PT-004 | M08/M11 | Workflow/approval and publish guard | Approval version and block tests |
| BR-013 | PR-PT-006 | M12 | Addendum versions/events/pages | Addendum impact E2E |
| BR-014 | PR-PT-007/008; SR-SUB | M17/M18 | Encryption, KMS, exchange, opening | Independent review and attack tests |
| BR-015/016 | PR-PT-009–011 | M19/M20 | Evaluation/award DB, APIs, pages | Evaluation-to-award UAT |
| BR-020/021 | PR-TP-004/005 | M14/M21 | Import, provenance, extraction | File/URL import and verification tests |
| BR-022 | PR-TP-001/002 | M13 | Profile/vault DB, API, pages | Expiry/version/access tests |
| BR-023/024 | PR-TP-008/011 | M16 | Requirement/evidence/readiness | Mandatory blocker UAT |
| BR-025 | PR-TP-009 | M16 | Proposal/pricing controls | Financial access matrix |
| BR-026 | PR-TP-012/013 | M16/M17 | Package, receipt/evidence | Exact-version reconciliation |
| BR-030–034 | PR-TP-003; SR-AI-005/006 | M15 | Connectors, index, search, alerts | Relevance/freshness/dedup benchmarks |
| BR-040–046 | PR-PT-003/010; PR-TP-005/010/011 | M21–M23 | AI jobs, retrieval, citations, UI | AI benchmark and human review UAT |
| BR-050–054 | PR-SH-008/010; SR-EVT/DATA | M06/M24/M25 | Audit, country packs, reports | Audit export and locale/country tests |

The project tracker expands this baseline to ticket level.

## 5. Acceptance criterion format

Use Given/When/Then for observable behavior and explicit non-functional thresholds where applicable.

Example:

```text
Requirement: RULE-SUB-003
Given a tender has not reached an authorized opening state
And the user is a procurement administrator
When the user requests a submission document or decryption release
Then the API denies access
And no object URL or key operation is issued
And the attempt is recorded as a security audit event
```

## 6. Evidence types

- Automated test report
- API contract test
- Migration verification
- Security/authorization test
- Performance report
- AI benchmark report
- Accessibility report
- Browser/device report
- Screenshot or recording of UAT flow
- Audit/evidence manifest
- Backup/restore report
- Architecture/security review
- Approved decision record
- Pilot sign-off

Screenshots alone do not prove backend rules or authorization.

## 7. Evidence register schema

Each evidence item records:

```yaml
id: EVID-W07-014
requirements:
  - BR-024
  - PR-TP-011
  - RULE-BID-003
release: R2
wave: W07
build: git-sha-or-release
environment: uat
executed_at: 2026-xx-xxTxx:xx:xxZ
executed_by: user-or-ci
artifact_paths:
  - reports/e2e/readiness.html
  - recordings/external-submission.webm
result: PASS
limitations: []
approvals:
  - role: Product Owner
  - role: QA Lead
```

## 8. Completion states

Use truthful states:

- `NOT_STARTED`
- `DESIGNED`
- `IMPLEMENTING`
- `IMPLEMENTED_NOT_VERIFIED`
- `VERIFIED_TECHNICALLY`
- `UAT_PENDING`
- `ACCEPTED`
- `DEFERRED`
- `BLOCKED`
- `REJECTED`

Do not use `COMPLETE` for work that lacks required verification or UAT.

## 9. Review and sign-off matrix

| Change | Required reviewers/sign-off |
|---|---|
| Business scope/rule | Product owner + domain/procurement owner |
| Tenant/data ownership | Architecture + security + data owner |
| Database migration | Backend/data reviewer + operations |
| API breaking change | API owner + frontend/integration owners |
| Authorization/security | Security reviewer + module owner |
| Sealed submission/opening | Security + procurement governance + architecture |
| AI use case/prompt/model | Product + AI owner + security/privacy where sensitive |
| Frontend workflow | Product/UX + accessibility + target persona UAT |
| Production release | Product + engineering + QA + operations + security as risk requires |

## 10. Change control

When a requirement changes:

1. Create decision/change record.
2. State reason, affected users, risks, and effective release.
3. Update BRD/PRD/SRD/business rules as needed.
4. Identify affected modules, data, APIs, pages, tests, prompts, reports, and documentation.
5. Re-estimate waves/tickets.
6. Preserve superseded requirement/version history.
7. Re-run impacted acceptance evidence.

Silent specification drift is prohibited.

## 11. Release acceptance package

Every release package contains:

- Scope and requirement list
- Known exclusions and deferred items
- Architecture/decision changes
- Migration and deployment plan
- Automated test summary
- Security and privacy review
- Performance and recovery evidence
- AI evaluation report where applicable
- UAT scripts/results/signatures
- Open defects and accepted risks
- Rollback/forward-fix and runbooks
- Release notes and support guidance

## 12. Product-level acceptance scenarios

### A. External tender

A Tenderpreneur discovers/imports, verifies, assesses, prepares, approves, audits, packages, submits externally, records evidence, and records outcome.

### B. Platform tender

A Procurement Team plans/authors/approves/publishes; Tenderpreneur prepares/submits; system seals; committee opens; evaluators score; approval awards; both sides receive outcome.

### C. Security isolation

Two tenants with similarly shaped records cannot discover or access each other's database rows, objects, search results, AI citations, jobs, exports, or exchange-private data.

### D. AI governance

AI produces source-linked assistance; user can inspect, edit, reject; unsupported content is labeled; final decision remains human and auditable.

### E. Recovery

A sampled tenant is restored to an isolated target, reconciled with objects/events, and passes critical workflow checks.

