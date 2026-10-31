# BID-DB-008 — Complete future procurement and exchange persistence design

## Metadata
- Ticket ID: BID-DB-008
- Epic: E-DB
- Type: Design
- Priority: P2
- Status: Blocked
- Complexity: L
- Dependencies: BID-ARC-009, BID-ARC-010, BID-API-009
- Blockers: Approved procedure-specific schemas and independent sealing review
- Milestone: M8

## 1. Objective
Close field-level future schema gaps before implementing marketplace persistence.
## 2. Business Context
Public projections, sealed receipts and evaluator records have different owners and disclosure rules.
## 3. Scope
Separate packets for requisition/tender versions/criteria, publication/addenda, exchange attempts/envelopes/receipts, opening authorization, evaluation locks and award/outcome lineage.
## 4. Out of Scope
Executing all migrations, cross-tenant foreign keys, plaintext sealed content.
## 5. Technical Requirements
Each packet defines columns/nulls/keys/checks/indexes, immutable publication criteria, authoritative acceptance lock order, replacement lineage, per-envelope release, independent score visibility and staged finance access. Define control/tenant/exchange recovery separately.
## 6. Proposed Implementation Approach
Trace approved commands to owner tables; resolve logical foreign references and consistency; enumerate race/recovery cases; obtain data/security review; split executable migration tickets per packet before development.
## 7. Dependencies
[Sealed policy](../architecture/BID-ARC-009.md), [future scope](../architecture/BID-ARC-010.md), [future contracts](../backend_contracts/BID-API-009.md).
## 8. Acceptance Criteria
- Given public projection, then internal notes/budgets are excluded by schema contract.
- Given duplicate accepted attempt, then approved unique key prevents double acceptance.
- Given early opening, then ordinary application credentials cannot retrieve plaintext/key capability.
## 9. Testing Requirements
Review constraint fixtures, timing/lock diagrams and restore/replay scenarios; no executed SQL claim.
## 10. Definition of Done
Approved packet schemas and migration child-task criteria; remaining policy blockers explicit.
## 11. References
[Database §§6/8](../../product/10_DATABASE_DESIGN.md), [schema gaps](../../technical_design/06_DATABASE_SCHEMA_DESIGN.md), [DB plan](../../implementation/05_DATABASE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Preliminary future table examples are not final executable DDL.
