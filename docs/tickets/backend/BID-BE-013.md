# BID-BE-013 — Design quorum-controlled opening and per-envelope release recovery

## Metadata
- Ticket ID: BID-BE-013
- Epic: E-BE
- Type: Design
- Priority: P2
- Status: Blocked
- Complexity: M
- Dependencies: BID-BE-012
- Blockers: Approved quorum/conflict/assurance and KMS release policy
- Milestone: M8

## 1. Objective
Specify opening commands that cannot release bids prematurely or conceal partial failures.
## 2. Business Context
Opening committee accountability and bid confidentiality must survive retries and key-service outages.
## 3. Scope
Session preparation/declarations, start/release/close, manifest verification, per-envelope release facts, minutes correction and operator recovery packets.
## 4. Out of Scope
Actual key release implementation, emergency bypass without governance, silent content release on scan/hash failure.
## 5. Technical Requirements
RULE-OPEN-001–004; authoritative closed/time state, quorum, conflict declarations and fresh step-up; ordinary procurement runtime no standing decrypt authority; immutable release/minutes; replay-safe per-envelope authorization.
## 6. Proposed Implementation Approach
Define committee/role matrix and KMS port; trace partial release failure; specify integrity-before-use and recovery status/UI; create separately reviewable command/adapter/test work packets.
## 7. Dependencies
[Sealed acceptance packet](BID-BE-012.md), including its approved API/schema/security prerequisites.
## 8. Acceptance Criteria
- Given too-early time, missing quorum or expired assurance, then no key operation is issued.
- Given partial release crash, then retry preserves prior release facts without hidden double release.
- Given tampered manifest, then content use stops with auditable failure.
## 9. Testing Requirements
Attack/negative matrix, KMS outage, hash mismatch, replay, conflicting committee member and immutable minutes fixtures.
## 10. Definition of Done
Approved opening packet and independent security/UAT plan; runtime remains blocked until child tickets exist.
## 11. References
[RULE-OPEN](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [security](../../technical_design/19_SECURITY_ARCHITECTURE.md), [security plan](../../implementation/15_SECURITY_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Human quorum cannot be replaced by a service account or AI recommendation.
