# BID-API-011 — Define plans, subscriptions, invoices and PesaPal contracts

## Metadata
- Ticket ID: BID-API-011
- Epic: E-API
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-001, BID-ARC-010, BID-INT-003
- Blockers: Per-plan storage/AI limits, invoice numbering/refund authority and PesaPal merchant/API decisions
- Milestone: M7

## 1. Objective
Freeze tenant-safe commercial contracts before plan, invoice or payment implementation.
## 2. Business Context
Tenderpreneurs need understandable plans and reliable payment without duplicate charges or premature access.
## 3. Scope
Approved KES plan/version catalogue, annual terms, included/additional licence prices, licence reservation/usage, monthly Bid allowance/usage, subscription changes, prorated additional-licence quote, invoice/receipt reads, payment initiation/return/status, PesaPal IPN intake and administrator reconciliation/adjustment operations.
## 4. Out of Scope
Procurement pricing, card-data handling by Bidora and provider status as sole entitlement authority.
## 5. Technical Requirements
Decimal money strings/currency and separately itemized VAT, server-side KES-minor-unit proration, effective versions, operation IDs, idempotency, expected versions, generic external failures, redacted provider references and audit/effect semantics.
## 6. Proposed Implementation Approach
Approve state matrices first; define OpenAPI and raw callback boundary; add duplicate/out-of-order/ambiguous examples and generated fixtures.
## 7. Dependencies
[Base API](BID-API-001.md), [billing design](../integrations/BID-INT-003.md), [Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md).
## 8. Acceptance Criteria
- Given duplicate payment initiation, then one logical invoice/payment operation is represented.
- Given pending provider status, then no paid entitlement is promised.
- Given exhausted Bid or licence allowance, then response provides a safe actionable limit error/quote path.
- Given one person with compatible multiple roles, then only one tenant licence is represented.
## 9. Testing Requirements
Contract tests for money, replay, stale plans, pending/reversed payment and cross-tenant reads.
## 10. Definition of Done
Approved commercial/API state packet and validated schemas; no payment implementation claimed.
## 11. References
[PR-PA-002/PR-SH-009](../../product/03_PRD.md), [gap audit](../14_PHASE_1_PORTAL_GAP_AUDIT.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Displayed return-page success is never authoritative settlement.

## 13. VAT/invoice/payment/refund authority packet — APPROVED; INVOICE SLICE IMPLEMENTED_NOT_VERIFIED

The requesting user approved C1–C15 in `backend/docs/commercial-invoice-payment-refund-decision-packet.md`. The bounded BID-API-011C1 provider-neutral invoice source now defines control migration 22, checked KES financial domain rules, an atomic invoice application/persistence boundary, audit/outbox evidence and `backend/api/openapi/drafts/commercial-financial-documents.json`. It seeds no tax profile, so issuance remains fail-closed; the service is unconfigured and no route or migration is active.

BID-API-011C2 adds an unmounted provider-neutral payment-operation and authoritative query-before-retry boundary at control source compatibility 23. BID-API-011C3 advances source compatibility to 24 and atomically converts one exact linked authoritative `SETTLED` observation into payment/invoice settlement, immutable receipt and the exact snapshotted plan or term-bound additional-licence effect. BID-API-011C4 advances source compatibility to 25 with Billing Administrator maker/distinct Platform Owner checker credit-note governance and durable provider-neutral refund intent; it neither calls a provider nor executes cash or entitlement disposition. BID-API-011C5 advances source compatibility to 26 with authority-bound settled-payment recheck requests and atomic exact-observation reversal handling: payment `REVERSED`, invoice `REVIEW_REQUIRED`, subscription `SUSPENDED` and an immutable retained-read-only hold commit together without rewriting original financial evidence. BID-API-011C6 adds unmounted exact tenant receipt/credit-note reads and audited AAL2 Platform Billing adjustment/reversal reads at source compatibility 26, with no rendering, provider evidence disclosure or routing selector. BID-API-011C7 adds bounded same-tenant invoice, redacted payment and combined financial-history keyset listings; it exposes no provider identifiers or offset pagination. BID-API-011C8 adds audited global Platform Billing invoice, payment, adjustment and reversal queues with exact capability/AAL2/no-affiliation checks and allowlisted filters, while accepting no tenant or routing selector. BID-API-011C9 adds separately registerable, unmounted Tenderpreneur and Platform Commercial Fiber boundaries for the implemented operations; settlement/reversal completion remains server-owned and has no browser route. These boundaries cannot interpret provider statuses. No PesaPal wire adapter, callback or status mapping was invented.

BID-API-011C10 is documentation-only **APPROVED POLICY — IMPLEMENTATION NOT AUTHORIZED** in `backend/docs/commercial-reversal-recovery-decision-contract.md`. RR1–RR15 define append-only maker/checker financial correction, exact-hold release and restoration of only an original effect's unexpired remainder through generation-fenced reconciliation. `ORIGINAL_FUNDS_REINSTATED` remains unavailable pending provider evidence and `REPLACEMENT_SETTLEMENT` remains unavailable pending finance/tax evidence. The user directed not to implement any RR path yet, so no source/schema/API authority follows from approval.

The umbrella ticket remains Blocked. PesaPal initiation/callback/query adapters, provider refund/reversal execution, reversal recovery, exact tax-profile activation, receipt rendering/download and live database/provider/entitlement-enforcement evidence are not complete. PesaPal still requires the exact contracted API/version, merchant onboarding, authenticity mechanism, supported KES methods and refund/reversal behavior. C1–C15 and RR1–RR15 policy approval do not satisfy those deployment/finance/tax/security gates or close BID-API-011.
