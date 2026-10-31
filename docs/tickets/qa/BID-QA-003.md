# BID-QA-003 — Verify accessible and responsive essential bid operations

## Metadata
- Ticket ID: BID-QA-003
- Epic: E-QA
- Type: Testing
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-008, BID-ARC-006
- Blockers: Runnable approved screens and target browser policy
- Milestone: M4

## 1. Objective
Validate WCAG 2.2 AA and mobile usability across accepted workflows.
## 2. Business Context
Approvals, uploads and deadline-sensitive evidence capture must remain usable beyond a desktop mouse.
## 3. Scope
Forms/error summaries, dialogs/focus, matrix headers/navigation, uploader/job live regions, source viewer, review/proof and outcome; browser/viewport/200% zoom coverage.
## 4. Out of Scope
Automated axe as sole evidence, pixel-perfect lock of dynamic data, native mobile application.
## 5. Technical Requirements
Keyboard-only and screen-reader manual checks; non-color status cues; reduced motion; readable long names/currency/timezone; accessible stacked/detail matrix alternative; safe network error recovery.
## 6. Proposed Implementation Approach
Run automated scans per critical route; perform manual persona journeys; file concrete owner-ticket defects; verify fixes and retain browser/device results.
## 7. Dependencies
[End-to-end screens](../frontend/BID-FE-008.md), [UX baseline](../architecture/BID-ARC-006.md).
## 8. Acceptance Criteria
- Given keyboard-only user, then evidence link and approval confirmation can be completed with visible focus.
- Given zoom/mobile, then essential actions remain reachable without information loss.
- Given async job error, then status is announced without misleading success.
## 9. Testing Requirements
axe reports, manual assistive-tech checklist, Playwright viewports and supported browser matrix.
## 10. Definition of Done
Material accessibility/usability defects resolved for release scope and manual evidence signed.
## 11. References
[UX-007](../../product/03_PRD.md), [components](../../technical_design/13_FRONTEND_COMPONENT_DESIGN.md), [QA plan](../../implementation/16_QA_TEST_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Virtualized tables require extra manual testing; render-speed alone is not acceptance.
