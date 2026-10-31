# BID-ARC-009 — Resolve future sealed acceptance, deadline and key-release design

## Metadata
- Ticket ID: BID-ARC-009
- Epic: E-ARC
- Type: Design
- Priority: P2
- Status: Proposed
- Complexity: L
- Dependencies: None
- Blockers: BID-ADR-009, BID-ADR-010; procurement/security approval
- Milestone: M8

## 1. Objective
Approve precise sealed-submission authority and recovery before any closed-loop implementation.
## 2. Business Context
Premature decryption or disputed acceptance time undermines procurement integrity.
## 3. Scope
Separate publication/window/closing/opening timestamps; equality/reservation/outage rules; serialized addendum/cancellation versus acceptance; durable attempt/receipt states; KMS key custody, quorum and recovery.
## 4. Out of Scope
External-MVP changes, cryptographic implementation, assuming encryption at rest constitutes sealing.
## 5. Technical Requirements
Specify acceptance linearization point, retry identities, receipt signing failure, object-orphan safety, bidder restore reconciliation and per-envelope release evidence. Ordinary procurement runtime must lack a standing pre-opening decrypt path.
## 6. Proposed Implementation Approach
Threat-model normal/failure timelines; compare key hierarchy/release options; obtain governance rules; define independent review/prototype evidence as follow-up authorized tasks; approve exact test oracle for cutoff boundaries.
## 7. Dependencies
None for design discovery. Independent review and explicit decisions are release blockers, not inferred from document completion.
## 8. Acceptance Criteria
- Given commit succeeded and response lost, then receipt recovery cannot create another submission.
- Given deadline equality or outage, then expected outcome has an approved rule.
- Given missing quorum/early opening, then no key release is possible under the reviewed design.
## 9. Testing Requirements
Fault-table review across storage/KMS/control/tenant/exchange; cryptographic threat model and exact boundary fixtures.
## 10. Definition of Done
Accepted temporal/recovery policy and independently reviewed crypto design, or explicit unresolved blockers retained.
## 11. References
[RULE-SUB/OPEN](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [system recovery](../../technical_design/01_SYSTEM_ARCHITECTURE.md), [security](../../technical_design/19_SECURITY_ARCHITECTURE.md), [security plan](../../implementation/15_SECURITY_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
L reflects review uncertainty; split prototype and independent assessment tasks when scope is approved.
