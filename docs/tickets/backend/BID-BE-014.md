# BID-BE-014 — Manage typed platform settings and country data

## Metadata
- Ticket ID: BID-BE-014
- Epic: E-BE
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-AUTH-003, BID-API-010, BID-DB-002
- Blockers: Secret-reference/KMS policy, setting approval rules and country/reference-data content validation
- Milestone: M7

## 1. Objective
Let authorized administrators draft, validate, activate and roll back typed global settings without exposing secrets.
## 2. Business Context
SMTP, AI, PesaPal and country configuration must be operationally changeable and auditable.
## 3. Scope
Typed versioned setting registry, environment scope, draft/validate/test-result/activate/supersede/rollback, secret references, countries/currencies/timezones/locales/holidays/reference versions, privacy-notice versions, retention-policy configuration and immutable audit/outbox.
## 4. Out of Scope
Provider adapter effects, generic arbitrary key/value writes, plaintext secret storage and tenant-private settings.
## 5. Technical Requirements
Per-setting schema validation; optimistic versions; one active compatible version; no secret readback; assurance/capability checks; activation failure leaves prior version effective.
## 6. Proposed Implementation Approach
Add reviewed control migration/query sets and type-specific application commands; provider tests occur through owner ports outside SQL transactions.
## 7. Dependencies
[Platform authority](../authentication/BID-AUTH-003.md), [contracts](../backend_contracts/BID-API-010.md), [control persistence](../database/BID-DB-002.md).
## 8. Acceptance Criteria
- Given invalid/untested configuration, then activation is rejected.
- Given failed activation, then the prior active version remains effective.
- Given any read/audit/log, then secret material is absent.
## 9. Testing Requirements
Concurrency, rollback, schema compatibility, secret redaction, country-version history and capability negatives.
## 10. Definition of Done
SMTP/AI/PesaPal reference and country setting lifecycles pass real DB and adapter-boundary tests.
## 11. References
[PR-PA-003](../../product/03_PRD.md), [Phase 1 settings](../../../backend/docs/phase-1-portal-functional-design.md), [privacy design](../../../backend/docs/phase-1-data-privacy-retention-deletion.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
A settings framework cannot weaken mandatory security invariants.
