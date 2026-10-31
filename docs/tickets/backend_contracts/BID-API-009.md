# BID-API-009 — Complete future procurement and exchange contract packets

## Metadata
- Ticket ID: BID-API-009
- Epic: E-API
- Type: Design
- Priority: P2
- Status: Blocked
- Complexity: L
- Dependencies: BID-API-001, BID-ARC-009, BID-ARC-010
- Blockers: Procedure policies and future field-level designs
- Milestone: M8

## 1. Objective
Replace endpoint-name-only future contracts with reviewed procedure-specific packets.
## 2. Business Context
Publication, sealed receipt, opening, independent scoring and award carry higher governance consequences than external tracking.
## 3. Scope
Approve packet boundaries for authoring/publish/addendum, participation/submit/replace/withdraw/receipt, quorum/release/minutes, evaluator scoring/lock/moderate and award/outcomes.
## 4. Out of Scope
Implementing all handlers in one ticket or including marketplace in first MVP.
## 5. Technical Requirements
Each packet needs request/read schemas, capabilities/conflict rules, publication version, exact deadline oracle, idempotency, stage/field filtering, audit and failure recovery. Split packet engineering work before implementation when policy remains incomplete.
## 6. Proposed Implementation Approach
Trace each command against approved domain/security policies; fill missing read endpoints and errors; review concurrency/receipt semantics; validate examples and compatibility independently by packet.
## 7. Dependencies
[Base](BID-API-001.md), [sealed/time rules](../architecture/BID-ARC-009.md), [future coverage](../architecture/BID-ARC-010.md).
## 8. Acceptance Criteria
- Given pre-opening procurement reader, then contracts expose metadata only.
- Given unpublished criterion, then scoring contract rejects it.
- Given response lost after acceptance, then receipt query has unambiguous semantics.
## 9. Testing Requirements
Contract fixtures for early/late cutoff, addendum race, quorum failure, financial-stage denial and locked score modification.
## 10. Definition of Done
Packet-specific approval evidence and child implementation-entry checklist; no runtime completion claim.
## 11. References
[Procurement/exchange APIs](../../product/11_BACKEND_API_DESIGN.md), [system recovery](../../technical_design/01_SYSTEM_ARCHITECTURE.md), [contracts plan](../../implementation/07_BACKEND_CONTRACTS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
L reflects discovery scope; future packets must be independently reviewed, not blindly generated.
