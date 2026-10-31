# BID-FE-010 — Deliver Platform Admin users and settings console

## Metadata
- Ticket ID: BID-FE-010
- Epic: E-FE
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-001, BID-API-010, BID-AUTH-003, BID-BE-014
- Blockers: Explicit web authorization, accepted platform capability/page contracts and secret-reference UX
- Milestone: M7

## 1. Objective
Let eligible Platform Administrators manage users, fixed roles and typed global settings through real APIs.
## 2. Business Context
Platform administration must change governed configuration safely rather than display read-only metadata.
## 3. Scope
Platform redirect/shell, administrator invite/grant/revoke, final-owner handling, SMTP/AI/PesaPal reference configuration, test/activate/rollback, countries/reference data and security-aware forms.
## 4. Out of Scope
Secret readback, tenant-private support content, commercial dashboard and retention execution screens.
## 5. Technical Requirements
Generated clients, TOTP step-up, explicit reason/version/idempotency, typed forms, no generic JSON editor, safe secret replacement indicators, accessible confirmation/conflict/recovery states.
## 6. Proposed Implementation Approach
Extend `web/` only after authorization; compose fixed-role and per-setting pages; display effective version/test status without exposing secret values.
## 7. Dependencies
[Shell](BID-FE-001.md), [contracts](../backend_contracts/BID-API-010.md), [authority](../authentication/BID-AUTH-003.md), [settings](../backend/BID-BE-014.md).
## 8. Acceptance Criteria
- Given unauthorized platform role, then neither navigation nor API reveals settings.
- Given failed configuration test, then activation is unavailable and prior version remains shown active.
- Given final-owner conflict, then the user receives an actionable safe explanation.
## 9. Testing Requirements
Playwright role/step-up/conflict/rollback/secret-bundle tests plus accessibility and mobile admin checks.
## 10. Definition of Done
Platform user and settings mutations work end to end with audit references and negative evidence.
## 11. References
[Phase 1 platform functions](../../../backend/docs/phase-1-portal-functional-design.md), [frontend design](../../technical_design/12_FRONTEND_ARCHITECTURE.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Browser state must never contain provider credentials.
