# BID-FE-011 — Deliver Platform Admin commercial and operations console

## Metadata
- Ticket ID: BID-FE-011
- Epic: E-FE
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-010, BID-API-011, BID-BE-016, BID-INT-004, BID-AUTH-004, BID-SEC-004
- Blockers: Explicit web authorization, commercial/retention/support decisions and accepted dashboard definitions
- Milestone: M7

## 1. Objective
Let authorized administrators operate plans, payments, support, retention and corrective dashboard actions.
## 2. Business Context
Phase 1 requires a working control plane for revenue and customer operations, not decorative charts.
## 3. Scope
Approved plan/licence/annual-price screens, subscriptions/invoices/payment reconciliation, tenant/usage/integration dashboards, support requests/sessions, privacy request operations, retention policy/legal-hold preview and eligible retry/pause/reconcile actions.
## 4. Out of Scope
Private tenant browsing outside support grant, arbitrary financial mutation and destructive retention without approved execution.
## 5. Technical Requirements
Permission-filtered counts, freshness labels, decimal/currency correctness, TOTP step-up, expected versions, idempotent action feedback, no private-content charts and accessible tables.
## 6. Proposed Implementation Approach
Build operation-focused pages over generated clients; link dashboard alerts to governed commands; distinguish pending external effects from success.
## 7. Dependencies
[Settings console](BID-FE-010.md), [commercial contracts](../backend_contracts/BID-API-011.md), [dashboard](../backend/BID-BE-016.md), [PesaPal](../integrations/BID-INT-004.md), [support](../authentication/BID-AUTH-004.md), [retention](../security/BID-SEC-004.md).
## 8. Acceptance Criteria
- Given pending payment, then UI never shows paid entitlement early.
- Given support expiry, then no private action remains available.
- Given dashboard alert, then corrective action rechecks current authority/source state.
## 9. Testing Requirements
Playwright plan/payment/support/retention/dashboard journeys, replay/conflict/provider outage, a11y and privacy tests.
## 10. Definition of Done
Commercial and operations journeys work against real dependencies and pass authorized persona UAT.
## 11. References
[PR-PA-001–005](../../product/03_PRD.md), [Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md), [privacy design](../../../backend/docs/phase-1-data-privacy-retention-deletion.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
A visual success state cannot substitute for reconciled provider or worker evidence.
