# BID-EVT-001 — Dispatch durable outbox events and commit inbox effects safely

## Metadata
- Ticket ID: BID-EVT-001
- Epic: E-EVT
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-BE-001, BID-API-007
- Blockers: Approved replay/retention and broker policies
- Milestone: M2

## 1. Objective
Implement at-least-once delivery without losing committed business events.
## 2. Business Context
Document scans and tenant provisioning cannot disappear when a worker or broker restarts.
## 3. Scope
Bounded outbox leases, NATS durable publish/ack, inbox/effect transaction, schema checks, subject ACLs and retry/dead-letter state.
## 4. Out of Scope
Exactly-once external-effect claims, all domain consumers, unaudited replay UI.
## 5. Technical Requirements
Stable event IDs; publish after commit; ack after effect commit; reject unknown schema; minimal classified payload; fair bounded tenant scheduling and cancellation.
## 6. Proposed Implementation Approach
Implement control-plane dispatch first and tenant discovery adapter later; fault-inject publish/commit/ack gaps; prove duplicate harmless DB effect; expose backlog metrics.
## 7. Dependencies
[Atomic boundary](../backend/BID-BE-001.md), [event schemas](../backend_contracts/BID-API-007.md).
## 8. Acceptance Criteria
- Given publish succeeds then dispatcher crashes, then retry repeats safely.
- Given consumer commits then loses ack, then repeat does not reapply effect.
- Given invalid schema, then message becomes visible quarantine/dead letter, not infinite retry.
## 9. Testing Requirements
Real NATS/PostgreSQL crash/replay/order tests, ACL denial and bounded connection scheduling.
## 10. Definition of Done
Failure-injection evidence, durable retry state and operational ownership recorded.
## 11. References
[SR-EVT](../../product/04_SRD.md), [events](../../technical_design/17_EVENT_MESSAGING_DESIGN.md), [worker plan](../../implementation/12_EVENTS_WORKERS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
External email/storage requires separate reconciliation beyond inbox dedup.
