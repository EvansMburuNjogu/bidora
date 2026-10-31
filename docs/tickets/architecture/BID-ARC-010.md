# BID-ARC-010 — Define future capability design packets and backlog expansion gates

## Metadata
- Ticket ID: BID-ARC-010
- Epic: E-ARC
- Type: Documentation
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-001
- Blockers: Phase 1 commercial/security decisions, accountable reviewers and explicit implementation authorization
- Milestone: M7

## 1. Objective
Close the selected Phase 1 Tenderpreneur and Platform Admin coverage gaps with explicit design owners and granular implementation-entry gates.
## 2. Business Context
The frozen external-tender source does not satisfy the selected shared portal, platform settings, organization knowledge, billing, PesaPal, support, retention or dashboard requirements.
## 3. Scope
Maintain the Phase 1 coverage register; resolve shared portal typing, simplified Tenderpreneur roles, fixed platform authority, typed settings, profile/knowledge, plans/subscriptions, PesaPal, controlled support, retention, dashboards, AI and UI/acceptance packets. Preserve separately deferred procurement publication/evaluation/award coverage.
## 4. Out of Scope
One ticket implementing all future modules, fabricated schemas or business policy, marking deferred scope complete.
## 5. Technical Requirements
For each packet list missing entities/fields, API/page contracts, policies, verification and owner. Require child engineering tickets before implementation where current designs are only preliminary. Existing future tickets below cover bounded operations; do not duplicate them.
## 6. Proposed Implementation Approach
Use the Phase 1 functional design and gap audit to review source coverage, approve missing commercial/security/provider decisions, and advance only bounded child tickets in dependency order.
## 7. Dependencies
[Scope alignment](BID-ARC-001.md). Future module authorization and packet acceptance precede its schema/API/handler work.
## 8. Acceptance Criteria
- Given a deferred PRD capability, then it has an owner, release candidate and design/verification gate.
- Given insufficient schema detail, then implementation is blocked and missing artifact named.
- Given existing ticket coverage, then no duplicate capability authority is created.
## 9. Testing Requirements
Source-to-ticket coverage audit and dependency-cycle validation after expansion.
## 10. Definition of Done
Reviewed future coverage register with bounded follow-up packets; no claim of runtime readiness.
## 11. References
[PRD](../../product/03_PRD.md), [Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md), [gap audit](../14_PHASE_1_PORTAL_GAP_AUDIT.md), [modules](../../product/07_MODULE_DEPENDENCIES.md), [quality gaps](../../technical_design/QUALITY_REPORT.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Comprehensive lifecycle coverage is not the same as exhaustive implementation detail for unapproved future procedures.
