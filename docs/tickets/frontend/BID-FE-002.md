# BID-FE-002 — Capture verified external tenders and display actionable bid workspace

## Metadata
- Ticket ID: BID-FE-002
- Epic: E-FE
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-001, BID-BE-002, BID-API-003
- Blockers: Approved page/source policy
- Milestone: M3

## 1. Objective
Deliver the first real tender workspace creation journey without AI.
## 2. Business Context
Bid managers need source/deadline clarity and an explicit human decision before assigning preparation work.
## 3. Scope
Manual capture/verification, opportunity detail, duplicate guidance, create/assign workspace, Bid/No-Bid rationale and filtered bid dashboard.
## 4. Out of Scope
Decorative fake statistics, automatic extraction, URL fetch.
## 5. Technical Requirements
API-003 typed client; exact source/version, zone/original date display; searchable member controls; unknown facts distinguished; version conflicts preserve draft; status/action hints come from server.
## 6. Proposed Implementation Approach
Implement forms/detail/list with URL filters and stable cursor; verify facts before conversion; connect real decision command; test slow/error/retry paths and responsive layout.
## 7. Dependencies
[Shell](BID-FE-001.md), [backend](../backend/BID-BE-002.md), [contracts](../backend_contracts/BID-API-003.md).
## 8. Acceptance Criteria
- Given verified input, then user creates and revisits a real tenant-private bid.
- Given ambiguous deadline or duplicate candidate, then UI requests explicit resolution.
- Given stale save, then user gets compare/refresh without silent overwrite.
## 9. Testing Requirements
Playwright capture-to-bid, date/owner validation, foreign record denial, cursor and keyboard/mobile tests.
## 10. Definition of Done
End-to-end first business slice with real data and audit references.
## 11. References
[PR-TP-004/006/007](../../product/03_PRD.md), [screen contracts](../../technical_design/12_FRONTEND_ARCHITECTURE.md), [frontend plan](../../implementation/10_FRONTEND_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
No input control should ask users to type raw database IDs.
