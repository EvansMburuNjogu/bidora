# BID-BE-016 — Serve actionable Phase 1 dashboards

## Metadata
- Ticket ID: BID-BE-016
- Epic: E-BE
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-BE-014, BID-TEN-004, BID-AUTH-004, BID-INT-004, BID-OBS-002
- Blockers: Metric definitions, freshness targets, permitted dimensions and action capability matrix
- Milestone: M7

## 1. Objective
Provide permission-filtered Tenderpreneur and Platform Admin dashboards with safe links to real corrective actions.
## 2. Business Context
Users need to know what requires attention and be able to act without fake statistics or private-data leakage.
## 3. Scope
Tenderpreneur plan/allowance/bids/deadlines/assignments/payments/support summaries; platform tenant/subscription/payment/integration/support/user/migration/security summaries; bounded drill-down and eligible retry/pause/reconcile links.
## 4. Out of Scope
Cross-tenant private-content analytics, predictive win claims and replacing operational telemetry.
## 5. Technical Requirements
Stable metric definitions, server-side permission filtering before counts, bounded freshness, no sensitive labels, cursor/drill-down contracts and action reauthorization.
## 6. Proposed Implementation Approach
Build source-owned projections/events; expose query ports and freshness; never join private tenant databases for platform aggregation.
## 7. Dependencies
[Settings](BID-BE-014.md), [subscriptions](../tenancy/BID-TEN-004.md), [support](../authentication/BID-AUTH-004.md), [payments](../integrations/BID-INT-004.md), [observability](../observability/BID-OBS-002.md).
## 8. Acceptance Criteria
- Given unauthorized scope, then neither row nor aggregate count leaks existence.
- Given stale projection, then freshness is visible and a mutation rechecks source truth.
- Given actionable failure, then retry remains idempotent and audited.
## 9. Testing Requirements
Cross-tenant counts, stale/out-of-order events, permission changes, bounded pagination and action recheck tests.
## 10. Definition of Done
Real-data dashboards and corrective actions pass privacy, isolation and freshness evidence.
## 11. References
[PR-PA-001–005](../../product/03_PRD.md), [observability](../../technical_design/23_OBSERVABILITY_OPERATIONS.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
BID-BE-016A adds unmounted permission-filtered Tenderpreneur and Platform Billing aggregation source as `IMPLEMENTED_NOT_VERIFIED`. It composes bounded recent owner-module reads, fails the whole aggregate on any denial/failure, exposes per-section `has_more`, and does not present bounded rows as global counts. Broader support/settings/security dashboards, routes, corrective actions, runtime composition and evidence remain blocked.

Operational metrics and commercial business records remain separate authorities.
