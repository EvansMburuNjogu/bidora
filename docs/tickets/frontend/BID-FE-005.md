# BID-FE-005 — Edit versioned technical responses and restricted pricing safely

## Metadata
- Ticket ID: BID-FE-005
- Epic: E-FE
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-004, BID-BE-004, BID-API-005
- Blockers: Approved editor/arithmetic policy
- Milestone: M4

## 1. Objective
Provide clear save/version/conflict interactions for technical and finance contributors.
## 2. Business Context
Silent overwrites or pricing leakage would undermine response preparation and later approval.
## 3. Scope
Structured section editor, revision history/compare, separate financial schedule, explicit currency and material-edit warning.
## 4. Out of Scope
Full office editor, localStorage draft database, frontend-owned arithmetic authority.
## 5. Technical Requirements
API-005 typed contracts; permitted content format; sanitization; decimal strings; expected version; independent financial route/data loading; safe unsaved-change navigation and no automatic stale resubmit.
## 6. Proposed Implementation Approach
Compose approved editor primitives; bind save/history; model server conflict; wire financial controls only for permitted capabilities; invalidate candidate/readiness queries after changes.
## 7. Dependencies
[Matrix](BID-FE-004.md), [response backend](../backend/BID-BE-004.md), [contracts](../backend_contracts/BID-API-005.md).
## 8. Acceptance Criteria
- Given technical-only identity, then pricing is absent from SSR payload and network responses.
- Given concurrent editor, then stale save cannot silently overwrite newer content.
- Given malicious pasted content, then preview is safe and validation explicit.
## 9. Testing Requirements
Editor/decimal component tests, Playwright multi-user conflict, XSS, financial field filtering and keyboard behavior.
## 10. Definition of Done
Real revision workflow with safe conflict and role-separated pricing evidence.
## 11. References
[PR-TP-009](../../product/03_PRD.md), [state design](../../technical_design/14_STATE_MANAGEMENT_DATA_FLOW.md), [frontend plan](../../implementation/10_FRONTEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Autosave is allowed only with visible safe version behavior, not last-write-wins.
