# BID-API-007 — Approve versioned event envelopes and effect-recovery contracts

## Metadata
- Ticket ID: BID-API-007
- Epic: E-API
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-001, BID-ARC-004
- Blockers: Event-owner and security review
- Milestone: M2

## 1. Objective
Define AsyncAPI-compatible events and replay rules before worker implementation.
## 2. Business Context
Provisioning, scanning, package jobs and notifications must survive restarts without duplicating business effects.
## 3. Scope
Envelope IDs/version/tenant/actor/correlation/classification; provisioning, membership, document, candidate, package and external-evidence event payloads; retry/dead-letter compatibility.
## 4. Out of Scope
Raw document/pricing payloads, treating actor ID as credential, exactly-once guarantees.
## 5. Technical Requirements
Minimal references, producer/consumer ownership, ordering/gap strategy, schema version policy, subject ACLs and dedup retention. Distinguish DB inbox commit from external operation reconciliation.
## 6. Proposed Implementation Approach
List each producer/effect; define schemas and out-of-order examples; review authorization re-resolution; document invalid-schema quarantine and audited replay.
## 7. Dependencies
[Base conventions](BID-API-001.md), [transaction ownership](../architecture/BID-ARC-004.md).
## 8. Acceptance Criteria
- Given repeated event ID, then effect contract identifies the dedup key.
- Given unknown schema version, then consumer must quarantine rather than loop indefinitely.
- Given revoked requester, then event presence cannot authorize private processing.
## 9. Testing Requirements
Schema examples, minimal-data review, compatibility fixtures and crash/ack timeline table.
## 10. Definition of Done
Reviewed AsyncAPI schemas and effect/idempotency table, ready for consumers only after dependencies accepted.
## 11. References
[SR-EVT-001–005](../../product/04_SRD.md), [events](../../technical_design/17_EVENT_MESSAGING_DESIGN.md), [worker plan](../../implementation/12_EVENTS_WORKERS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Expiring inbox records too early can re-enable old irreversible effects.
