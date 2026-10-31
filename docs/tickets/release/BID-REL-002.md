# BID-REL-002 — Onboard controlled pilot teams and validate feedback against evidence

## Metadata
- Ticket ID: BID-REL-002
- Epic: E-REL
- Type: Testing
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-REL-001, BID-OPS-003
- Blockers: Approved pilot participants/data terms and support ownership
- Milestone: M5

## 1. Objective
Validate actual bid-team usability and reliability under controlled exposure.
## 2. Business Context
Pilot users reveal operational blockers that synthetic success paths may miss.
## 3. Scope
Approved pilot onboarding, tenant isolation/configuration check, roles/training, privacy notices, scoped feature flags, support/runbook handoff, feedback and defect triage.
## 4. Out of Scope
Unrestricted public launch, real data before consent/retention approval, automatically accepting suggested procurement rules.
## 5. Technical Requirements
Document participant/data authority; default least privilege; service/support boundaries; agreed recovery/incident communication; collect minimized telemetry and task-linked feedback; avoid private evidence in recordings.
## 6. Proposed Implementation Approach
Select pilot cohort; verify production-like safeguards; run representative workflows; monitor incidents and time-to-operation; turn findings into bounded reviewed tickets; retest fixes before expanding cohort.
## 7. Dependencies
[MVP acceptance](BID-REL-001.md), [recovery](../devops/BID-OPS-003.md).
## 8. Acceptance Criteria
- Given pilot tenant, then onboarding grants only intended roles and private storage/DB.
- Given blocker feedback, then owner/severity/reproduction and acceptance test are recorded.
- Given incident, then support follows approved containment/communication procedure.
## 9. Testing Requirements
Pilot UAT records, scoped monitoring, role/access checks, feedback-to-fix regression evidence.
## 10. Definition of Done
Pilot findings dispositioned and user/operations acceptance recorded; remaining issues explicit.
## 11. References
[Roadmap R2](../../product/16_PRODUCT_ROADMAP.md), [operations design](../../technical_design/23_OBSERVABILITY_OPERATIONS.md), [release plan](../../implementation/19_RELEASE_READINESS.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Pilot access is controlled exposure, not proof of generalized production capacity.
