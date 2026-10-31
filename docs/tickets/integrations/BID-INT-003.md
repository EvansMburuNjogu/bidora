# BID-INT-003 — Design PesaPal invoice, payment and entitlement reconciliation

## Metadata
- Ticket ID: BID-INT-003
- Epic: E-INT
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-010, BID-TEN-003
- Blockers: PesaPal API/version and merchant policy; invoice numbering/refund authority/tax deployment validation; missing billing schema
- Milestone: M7

## 1. Objective
Complete the PesaPal-specific billing design before implementing payment initiation, IPN handlers or entitlement effects.
## 2. Business Context
Payment notifications cannot directly grant paid capabilities or accidentally charge bid teams twice.
## 3. Scope
Approved plan/licence/subscription/invoice/payment/refund states, proration and VAT line items, PesaPal order and IPN fields, callback authenticity, raw-body preservation where required, deduplication, reconciliation and entitlement effective-version mapping.
## 4. Out of Scope
Actual PesaPal adapter, invented plan values, browser-declared payment success and using an unverified provider callback as product authority.
## 5. Technical Requirements
Define control-plane tables/constraints, operation IDs, out-of-order/replayed callback behavior, uncertain charge recovery, retention and financial audit. Specify request/read/error schemas and tenant-safe usage/invoice UI before child implementation.
## 6. Proposed Implementation Approach
Verify the contracted PesaPal API and sandbox behavior; approve commercial rules; map state transitions and ambiguous outcomes; specify reconciliation queries and child API/runtime/UI tests.
## 7. Dependencies
[Future design coverage](../architecture/BID-ARC-010.md), [entitlements](../tenancy/BID-TEN-003.md).
## 8. Acceptance Criteria
- Given duplicate signed webhook, then the design permits only one financial effect.
- Given ambiguous charge timeout, then query/reconcile precedes retry charge.
- Given invalid signature or stale event, then entitlement cannot advance.
## 9. Testing Requirements
Sandbox fixture/test plan for forged/replayed/out-of-order events and cross-tenant invoice reads.
## 10. Definition of Done
Approved billing schema/contracts/state matrix and executable child-ticket requirements, not implemented payment claims.
## 11. References
[PR-SH-009](../../product/03_PRD.md), [Phase 1 commercial design](../../../backend/docs/phase-1-portal-functional-design.md), [runtime adapter](BID-INT-004.md), [integrations](../../technical_design/16_INTEGRATION_ARCHITECTURE.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
The Phase 1 plan/licence prices are product-approved, but PesaPal contractual/API behavior, tax treatment and refund authority still require deployment/commercial validation. Manual plan assignment is not billing completion.

## 13. Commercial policy continuation — DRAFTED_PENDING_APPROVAL

BID-API-011C at `backend/docs/commercial-invoice-payment-refund-decision-packet.md` is approved. Its bounded provider-neutral invoice, query-before-retry payment/reconciliation, atomic settlement and maker/checker credit-note/refund-intent schema/domain/application source is implemented-not-verified and unmounted, with no seeded tax profile or provider adapter. C4 records only an approved refund intent; it performs no PesaPal refund/query, cash settlement or entitlement disposition. C5 can schedule a settled payment for the existing provider-neutral query boundary and atomically fail closed on an exact reconciled reversal, but it adds no PesaPal adapter, wire status mapping, callback or query implementation. It deliberately does not invent a tax rate or PesaPal API/authenticity contract. BID-INT-003 remains blocked until exact contracted PesaPal sandbox/API/authenticity evidence permits provider-specific schema/callback design and accountable finance/tax owners activate an immutable tax profile.
