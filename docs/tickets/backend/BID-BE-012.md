# BID-BE-012 — Design sealed acceptance, withdrawal and replacement command packets

## Metadata
- Ticket ID: BID-BE-012
- Epic: E-BE
- Type: Design
- Priority: P2
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-009, BID-DB-008, BID-BE-009, BID-API-009
- Blockers: Independent crypto and exact acceptance semantics
- Milestone: M8

## 1. Objective
Turn the reviewed sealing architecture into bounded implementation packets for each irreversible command.
## 2. Business Context
Bidder retries and amendments must not create duplicate or disputed accepted bids.
## 3. Scope
Attempt preparation, authoritative acceptance/receipt query, permitted withdrawal/replacement lineage, acknowledgement reconciliation and orphan cleanup safety.
## 4. Out of Scope
Runtime encryption, one giant sealed-service ticket, first external MVP.
## 5. Technical Requirements
Recheck publication/state/time/authority at approved linearization point; immutable candidate; scoped idempotency; no plaintext keys in DB/logs; exchange acceptance/audit/outbox atomic; signing-pending semantics; replacement preserves history.
## 6. Proposed Implementation Approach
Define lock/lease ordering and failure checkpoints; specify child adapter/command/receipt/reconciliation tasks and security evidence; validate timelines against approved deadline policy.
## 7. Dependencies
[Security decision](../architecture/BID-ARC-009.md), [schema](../database/BID-DB-008.md), [publication](BID-BE-009.md), [contracts](../backend_contracts/BID-API-009.md).
## 8. Acceptance Criteria
- Given accepted commit with lost response, then retry queries original receipt.
- Given addendum/cancellation race, then serialization enforces one approved outcome.
- Given replacement, then prior envelope/receipt remains immutable.
## 9. Testing Requirements
Boundary-time, KMS/storage/DB outage, replay/tamper, restored bidder and duplicate receipt fixtures.
## 10. Definition of Done
Reviewed implementable per-command packets and independent-review evidence references, not handlers.
## 11. References
[RULE-SUB](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [recovery design](../../technical_design/01_SYSTEM_ARCHITECTURE.md), [security plan](../../implementation/15_SECURITY_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Outbox does not make object/KMS/exchange effects one distributed transaction.
