# BID-INT-004 — Reconcile PesaPal payments without duplicate effects

## Metadata
- Ticket ID: BID-INT-004
- Epic: E-INT
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-INT-003, BID-API-011, BID-TEN-004
- Blockers: PesaPal API/version, merchant sandbox, callback authenticity, tax/invoice/refund and credential lifecycle validation
- Milestone: M7

## 1. Objective
Initiate and reconcile PesaPal payments while applying each financial and entitlement effect at most once.
## 2. Business Context
Tenderpreneurs must be able to pay for a plan without duplicate charges or false paid status.
## 3. Scope
Explicit provider config, immutable invoice line items for plan/additional licences/VAT/credits, order creation, redirect/return, IPN registration/intake, authoritative status query, deduplication, pending/failure/reversal/refund reconciliation, receipt evidence and admin retry.
## 4. Out of Scope
Storing card/mobile-money credentials, browser-declared success and additional payment providers.
## 5. Technical Requirements
Secret references only; verified TLS; bounded non-redirecting service transport where applicable; raw callback authenticity; provider/reference uniqueness; ambiguous timeout query-before-retry; atomic invoice/payment/outbox transitions.
## 6. Proposed Implementation Approach
Implement provider port and PesaPal adapter against approved sandbox; isolate callback verification; reconcile through leased jobs and effect keys.
## 7. Dependencies
[Provider design](BID-INT-003.md), [contracts](../backend_contracts/BID-API-011.md), [subscriptions](../tenancy/BID-TEN-004.md).
## 8. Acceptance Criteria
- Given replayed/out-of-order IPN, then one valid state progression/effect occurs.
- Given timeout after order creation, then status is queried before another charge attempt.
- Given invalid callback, then invoice and entitlement do not advance.
## 9. Testing Requirements
PesaPal sandbox/fixture signatures, duplicates, reordering, timeout, reversal/refund, credential rotation and cross-tenant reads.
## 10. Definition of Done
Sandbox payment-to-entitlement journey and reconciliation runbook have executed evidence; production acceptance remains separate.
## 11. References
[PR-SH-009](../../product/03_PRD.md), [integration design](../../technical_design/16_INTEGRATION_ARCHITECTURE.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Provider behavior and supported authenticity mechanisms must be verified from the contracted PesaPal API, not assumed.

## 13. Commercial policy dependency — DRAFTED_PENDING_APPROVAL

`backend/docs/commercial-invoice-payment-refund-decision-packet.md` C1–C15 is approved, and its provider-neutral immutable-invoice, query-before-retry payment/reconciliation and atomic settlement/receipt/entitlement source is implemented-not-verified and unmounted. Provider deadlines, uncertainty delay and retry schedule are mandatory explicit configuration with no defaults. This runtime ticket remains blocked pending verification of the exact contracted PesaPal API/version, merchant sandbox, authenticity mechanism, supported KES methods, status mapping and refund behavior. No provider adapter, callback schema, payment effect, refund or entitlement activation is inferred from policy approval or the provider-neutral tables.
