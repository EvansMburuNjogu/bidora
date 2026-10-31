# BID-TEN-004 — Operate plans, subscriptions and monthly Bid allowance

## Metadata
- Ticket ID: BID-TEN-004
- Epic: E-TEN
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-TEN-003, BID-API-011, BID-AUTH-003
- Blockers: Per-plan storage/AI limits, invoice numbering/refund authority and deployment tax validation
- Milestone: M7

## 1. Objective
Provide versioned Tenderpreneur plans, subscriptions and race-safe monthly Bid-workspace entitlement.
## 2. Business Context
Customers need clear limits and upgrades while retained bids remain available after limits or subscription changes.
## 3. Scope
Approved monthly/annual KES plan versions, publish/retire, 14-day trial, included/additional licences, subscriptions, upgrades/downgrades/cancel renewal, server-side prorated mid-period licence additions, credits/temporary adjustments, usage query and reserve/settle/release for invitations/memberships and successful `BID` workspace creation.
## 4. Out of Scope
PesaPal transport, procurement plans, destructive deletion and silent historical repricing.
## 5. Technical Requirements
Decimal money/currency, VAT as a separate invoice line, immutable historical terms, effective dates, backend enforcement, atomic final-capacity races, failed-operation release, no Bid rollover/archive refund, no automatic user removal on downgrade and approved suspended remediation.
## 6. Proposed Implementation Approach
Add reviewed control migrations/queries; implement commercial state machine and entitlement port; connect workspace creation through operation IDs.
## 7. Dependencies
[Entitlements](BID-TEN-003.md), [contracts](../backend_contracts/BID-API-011.md), [platform authority](../authentication/BID-AUTH-003.md).
## 8. Acceptance Criteria
- Given two requests for the final allowance, then at most one workspace settles usage.
- Given failed creation, then reservation releases exactly once.
- Given plan version change, then prior invoice/subscription terms remain intact.
- Given mid-period additional licence, then the reviewed KES-minor-unit proration is deterministic and entitlement waits for reconciled payment.
- Given downgrade below active/reserved licences, then no user is removed and the change remains blocked/actionable.
## 9. Testing Requirements
Period-boundary/timezone, concurrency, retry, upgrade/downgrade, cancellation, suspension and cross-tenant tests.
## 10. Definition of Done
Plan-to-workspace journey and recovery pass real PostgreSQL evidence with audited platform actions.
## 11. References
[PR-PA-002](../../product/03_PRD.md), [Phase 1 plans](../../../backend/docs/phase-1-portal-functional-design.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
BID-TEN-004F adds an unmounted current paid-write guard, guarded Opportunity/capture authorization adapters and a strengthened pre-provider AI reservation recheck as `IMPLEMENTED_NOT_VERIFIED`. Active payment-reversal holds and other retained-read-only states deny new paid mutations without blocking authorized reads or ownership-continuity administration.

BID-API-011C10 records RR1–RR15 as **APPROVED POLICY — IMPLEMENTATION NOT AUTHORIZED**: an exact reversal hold remains active through maker/checker decision and generation-fenced reconciliation; any restoration is limited to the original unexpired effect without allowance reset or term extension and cannot override another hold. `ORIGINAL_FUNDS_REINSTATED` and `REPLACEMENT_SETTLEMENT` remain unavailable behind provider and finance/tax gates, and no `REVERSAL_CONFIRMED` or other hold-release/restoration implementation is authorized. Runtime composition and cross-plane race evidence remain excluded.

The approved product catalogue is Trial 1 Bid/2 licences; Basic KES 4,500 with 3 Bids/3 licences; Growth KES 12,500 with 10/8; Professional KES 29,500 with 30/20; Enterprise from KES 60,000. Extra licences and annual prices follow the approved Phase 1 design; deployment tax/legal validation still applies.
