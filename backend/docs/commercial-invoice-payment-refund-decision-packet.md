# BID-API-011C — VAT, invoice, PesaPal payment and refund authority decision packet

Status: **C1–C15 APPROVED — BOUNDED PROVIDER-NEUTRAL INVOICE, PAYMENT-RECONCILIATION, SETTLEMENT, ADJUSTMENT AND REVERSAL SOURCE IMPLEMENTED_NOT_VERIFIED**  
Scope: Phase 1 KES invoicing, additional-licence proration, PesaPal payment reconciliation, receipts, credits and refunds  
Personas: Tenderpreneur Organization Administrator purchasing service; Billing Administrator reconciling financial records; distinct Platform Owner approving adjustments/refunds  
Not implementation authority: no tax rate, production merchant account, PesaPal API/version/authenticity mechanism, schema, migration, provider call, route, payment, refund or entitlement activation is approved by this draft alone.

## 1. Outcome and boundaries

This packet closes product authority and state-machine questions while preserving deployment-owned tax/provider gates.

Included:

- VAT-exclusive price and immutable tax-profile representation;
- exact KES minor-unit arithmetic and rounding profile;
- immutable invoice, credit-note and receipt numbering/lifecycle;
- additional-licence proration for monthly and annual subscriptions;
- tenant payment initiation and provider-neutral PesaPal reconciliation;
- duplicate, out-of-order, timeout, reversal and refund behavior;
- Billing Administrator maker and Platform Owner checker boundaries;
- exact entitlement effects and financial evidence.

Excluded:

- selecting or asserting the Kenyan VAT rate or taxability without qualified tax approval;
- tax filing, eTIMS/e-invoicing integration, withholding tax or fiscal-device behavior;
- storing card, mobile-money or bank credentials;
- production PesaPal credentials, merchant onboarding, endpoint/version, callback authenticity or payment-method promises;
- accepting browser return pages or callbacks as settlement authority;
- cash/manual/off-platform payment recording;
- foreign currencies or currency conversion;
- chargebacks beyond provider-reported reversal reconciliation;
- frontend and runtime activation.

All commercial money is exact KES minor units. A provider reference, redirect, callback, invoice status or Platform Admin role never independently grants a paid entitlement.

## 2. Decision C1 — tax profile and VAT boundary

**Recommended policy:** Catalogue prices remain VAT-exclusive as already approved. Every issued taxable invoice snapshots one immutable, finance/tax-reviewed tax profile containing:

- profile UUID/version and applicability (`KE`, KES, customer/tax category);
- tax code and safe display label;
- exact non-negative rational rate represented as integer numerator/denominator with bounded values;
- rounding profile;
- effective start/end instants;
- legal/tax-review evidence reference and approved content hash;
- state `DRAFT`, `VALIDATED`, `SCHEDULED`, `ACTIVE`, `SUPERSEDED` or `WITHDRAWN`.

Bidora has **no default VAT rate**. This packet does not approve `16%` or any other rate. If no exact ACTIVE reviewed tax profile applies at invoice issue time, paid invoice creation and payment initiation fail closed with `commercial_tax_profile_unavailable`.

Tax applicability and exemptions are not inferred from a free-form tax identifier. A zero-rated/exempt profile requires separately reviewed evidence and an explicit tax code; missing information does not become zero VAT.

The invoice shows subtotal, discounts/credits applied before tax where the approved profile permits, taxable base, VAT by tax code and total. `prices_include_vat` remains false. Historical invoices retain their snapshotted tax profile even after a later rate change.

## 3. Decision C2 — money arithmetic and rounding

**Recommended policy:**

- Internal and persisted amounts use signed 64-bit KES minor units with checked overflow.
- Charge/invoice line unit price, quantity basis, net, tax and gross are persisted separately; API amounts are strict decimal strings with exactly two digits.
- Ordinary charge quantities are positive integers. Proration uses a separately persisted rational factor, never binary floating point.
- Tax is calculated per tax-code aggregate after exact net-line calculation: `round_half_up(taxable_minor_units × rate_numerator / rate_denominator)`.
- Invoice total is `subtotal - invoice_level_credit + VAT`; it must be non-negative and exactly equal the sum of persisted components.
- A fractional minor unit rounds half away from zero for charges and credit notes under the same snapshotted profile. No line silently truncates.
- Every calculation stores arithmetic profile `KES_COMMERCIAL_V1` and canonical calculation inputs/hash.

The rounding profile itself requires finance/tax approval with C1. A later profile never recalculates an issued document.

## 4. Decision C3 — additional-licence quantity and proration

**Recommended policy:** One paid licence is one available active-person capacity above the plan's included licences. Compatible multiple roles for the same active member still consume one licence. Active accepted memberships and active invitations reserve capacity under the already approved rule; Platform and support users do not consume it.

An Organization Administrator purchases additional capacity before issuing an invitation that would exceed current paid capacity. No invitation creates a charge automatically.

The additional-licence catalogue price is monthly and receives no unapproved annual discount.

### Monthly base subscription

For a licence added during the current commercial period:

`prorated_minor = ceil(monthly_licence_minor × remaining_period_nanoseconds / total_period_nanoseconds)`

using the immutable UTC commercial-period boundaries and invoice-issue instant. A positive addition costs at least one minor unit. The licence expires with the current term and renews at the then-approved full monthly price unless reduced before renewal.

### Annual base subscription

The charge equals:

- the same prorated amount for the remaining portion of the current monthly usage period; plus
- one full monthly additional-licence price for each later complete usage month before the annual commercial-period end.

There is no annual additional-licence discount in Phase 1. Renewal quotes charge the snapshotted additional-licence monthly price times twelve for each retained additional licence, unless a later approved catalogue says otherwise.

A quote binds exact subscription/term/catalogue/tax/usage-period versions, quantity, calculation instant and commercial-period end. It expires after 15 minutes. Invoice issue recomputes against current versions and may differ from an expired quote. Once an invoice is issued, its amount is immutable; payment delay does not create a retroactive refund or extension. Additional capacity becomes effective only after exact payment reconciliation and ends at the snapshotted term boundary.

Removing/reducing licences changes renewal quantity only. It produces no automatic mid-period credit/refund and cannot reduce capacity below active membership/invitation reservations.

## 5. Decision C4 — immutable invoice and numbering policy

**Recommended policy:** Invoice states are:

- `DRAFT`;
- `ISSUED`;
- `PAYMENT_PENDING`;
- `PAID`;
- `PARTIALLY_CREDITED`;
- `CREDITED`;
- `VOID`;
- `REVIEW_REQUIRED`.

A draft has a UUID but no legal document number. Final issue atomically allocates a number, snapshots seller/customer billing facts, subscription term, line items, tax profile/calculation and due instant, then makes content immutable.

Proposed Phase 1 number formats are:

- invoice: `BDR-INV-YYYY-NNNNNNNN`;
- credit note: `BDR-CRN-YYYY-NNNNNNNN`;
- payment receipt: `BDR-RCT-YYYY-NNNNNNNN`.

`YYYY` uses the calendar year in `Africa/Nairobi` at authoritative issue time. Each document type has a separate transactional counter. Numbers are allocated only in the commit that issues the document, never reused, and retained when a document is voided. Bidora does not promise a legally “gapless” sequence until finance/tax review confirms the counter/recovery behavior.

A final issued invoice is never edited or deleted. Before payment, a materially incorrect invoice is voided with reason and a replacement invoice is issued. After payment, corrections use immutable credit notes and, if needed later, separately approved debit notes; history remains linked.

Invoice lines are allowlisted: `PLAN_BASE`, `ADDITIONAL_LICENCE`, `APPROVED_CREDIT`, and `VAT`. Arbitrary administrator-authored line descriptions/amounts cannot be used to change a catalogue charge. Enterprise contracted lines require an exact approved immutable contract term.

## 6. Decision C5 — billing snapshot and invoice visibility

**Recommended policy:** An issued invoice snapshots only required billing facts:

- seller legal/trading name, tax identifier and approved billing address from deployment-owned finance configuration;
- tenant legal name, optional reviewed tax identifier and billing address from the authorized organization profile;
- invoice/due dates, currency, line/tax totals and immutable term references;
- no password, session, membership list, tender data, document content, private pricing beyond the tenant's own commercial terms or provider credential.

Organization Administrators with current `tenderpreneur.organization.subscription.read` may list/read their tenant's invoices, credit notes, receipts and redacted payment state. `tenderpreneur.organization.subscription.manage` authorizes quote/invoice/payment initiation for that same tenant after current membership/context checks.

Billing Administrators require `platform.invoice.read` or `platform.payment.read` and current AAL2. Platform routes accept no tenant/cell/database selector beyond typed control-plane invoice/payment filters; provider references are redacted by default. Invoice access is audited and private/no-store.

## 7. Decision C6 — invoice issue and payment initiation

**Recommended policy:** A tenant payment operation:

1. rechecks active Tenderpreneur tenant, current Organization Administrator authority and exact context;
2. locks subscription/term/quantity/catalogue/tax-profile facts;
3. issues or reuses one exact unpaid invoice for the canonical purchase operation;
4. creates one logical payment operation keyed by invoice and UUID idempotency;
5. calls PesaPal outside the SQL transaction using a provider idempotency/merchant-reference value where the contracted API supports it;
6. records the provider outcome through a second fenced transaction;
7. returns a redirect only for a confirmed created provider order.

The client cannot supply amount, currency, invoice number, callback URL, merchant reference, tenant database, success state or entitlement terms.

Only one nonterminal payment operation may exist per invoice. Matching initiation replay reauthorizes and returns the existing safe payment status/redirect if still valid. Same key with changed purchase facts conflicts.

A redirect is short-lived and returned only from an allowlisted contracted PesaPal origin. Bidora never proxies payment credentials or embeds provider secrets.

## 8. Decision C7 — provider-specific gate

**Recommended policy:** PesaPal remains the selected Phase 1 provider, but adapter/runtime implementation stays blocked until deployment records and verifies:

- exact contracted API product and version;
- sandbox and production base endpoints;
- merchant/account onboarding status;
- credential and IPN-registration secret references;
- exact order-creation idempotency behavior;
- exact callback/IPN authenticity mechanism over raw bytes/fields;
- authoritative transaction-status query and status mapping;
- provider reference constraints and uniqueness;
- supported KES payment methods;
- redirect/return behavior;
- timeout, rate, retry and reconciliation constraints;
- refund API/support and reversal semantics;
- data-processing region, retention and contractual evidence.

No source may guess a signature header, shared-secret algorithm, replay window or provider status. If the contracted API offers no cryptographic callback authentication, IPN is treated only as an untrusted reconciliation trigger and an authenticated server-to-server status query is mandatory before any financial transition.

Secrets are write-only references under the approved typed-settings/maker-checker model. Provider configuration cannot activate without a synthetic sandbox test and distinct Platform Owner approval.

## 9. Decision C8 — payment states and ambiguous outcomes

**Recommended policy:** Payment-operation states are:

- `CREATED`;
- `PROVIDER_CREATE_PENDING`;
- `AWAITING_CUSTOMER`;
- `PROVIDER_PENDING`;
- `SETTLED`;
- `FAILED`;
- `CANCELLED`;
- `REVERSED`;
- `REFUND_PENDING`;
- `PARTIALLY_REFUNDED`;
- `REFUNDED`;
- `REVIEW_REQUIRED`.

A timeout or connection failure after order-create dispatch is `PROVIDER_CREATE_PENDING`, not `FAILED`. Bidora queries by its immutable merchant/operation reference before another create attempt. It never blindly retries a possibly effective charge.

Browser return is navigation only. It may show `processing` and trigger a bounded status refresh, but cannot set invoice/payment/subscription state.

IPN/callback intake validates body size/content type and preserves only the bounded evidence required by the contracted authenticity mechanism. It deduplicates provider event/reference facts and schedules reconciliation. Invalid, unverifiable, malformed, stale or foreign events produce no invoice or entitlement mutation.

Out-of-order provider observations append history. State transitions use an explicit precedence table and authoritative query timestamp; an older `PENDING` cannot overwrite reconciled `SETTLED`, `REVERSED` or `REFUNDED` state.

## 10. Decision C9 — settlement and entitlement activation

**Recommended policy:** An invoice becomes `PAID` and a payment becomes `SETTLED` only after an authenticated authoritative PesaPal status query proves:

- the exact provider order/reference maps to the expected Bidora payment operation;
- merchant/account and currency are expected;
- settled KES amount equals the invoice outstanding total exactly;
- state is terminal-success under the verified provider mapping;
- the financial effect has not already committed.

Amount mismatch, reference collision, unsupported currency or conflicting terminal status becomes `REVIEW_REQUIRED`; no entitlement is activated.

Because invoice, payment, subscription and entitlement authority live in the control plane, exact settlement commits in one control transaction:

- payment observation/transition;
- invoice paid state and outstanding amount;
- immutable receipt allocation;
- immutable paid subscription/additional-licence term effect;
- usage/allowance projection where applicable;
- audit, outbox and idempotency/effect evidence.

A callback, browser return, routing metadata, receipt generation or provider “accepted” create response has no entitlement effect. Settlement replay returns the one committed effect.

## 11. Decision C10 — renewal, grace, reversal and access

**Recommended policy:**

- Renewal invoices are issued against exact future terms before the current boundary under a separately scheduled operation.
- Failure to reconcile renewal by the boundary enters the existing three-day `PAST_DUE` grace; no duplicate renewal invoice is created for the same term.
- After grace, subscription becomes `SUSPENDED`/`RETAINED_READ_ONLY` under the approved lifecycle.
- A reconciled payment resumes only the exact paid term and never restores stale capabilities/contexts without current authorization checks.

A provider-reported reversal or chargeback is authoritative only after server query/reconciliation. It never deletes tenant data or rewrites the original receipt. It creates immutable reversal evidence, moves the payment to `REVERSED`, marks the invoice `REVIEW_REQUIRED`, and moves the affected paid subscription to a fail-closed review/suspension projection that blocks new metered/business writes while retaining authorized read/export/remediation paths. It must not use `LAST_RECONCILED` to continue full paid access.

The exact domain transition needed to distinguish financial reversal from transient reconciliation review must be implemented before payment runtime; existing generic `REVIEW_REQUIRED` behavior is insufficient if it preserves full access.

## 12. Decision C11 — credit, adjustment and refund authority

**Recommended policy:** Tenants may request a billing review but cannot issue credits or refunds.

A Billing Administrator with `platform.billing.adjustment.request`, current AAL2 and TOTP no older than 15 minutes may propose one exact immutable adjustment:

- `SERVICE_CREDIT`;
- `DUPLICATE_PAYMENT_REFUND`;
- `SERVICE_FAILURE_REFUND`;
- `PAYMENT_REVERSAL_CORRECTION`;
- `OTHER_CONTRACTUAL_REFUND`.

The proposal binds tenant, invoice, settled payment, maximum refundable amount, requested amount, tax treatment, reason, evidence references, entitlement disposition and expected financial versions. It cannot exceed the unrefunded settled amount, create a negative invoice balance, refund Trial, or target another merchant/currency.

Every credit/refund requires a distinct effective Platform Owner with `platform.governance.approve` and recent TOTP within 30 minutes of proposal, consistent with approved P6. Maker and checker differ and cannot be an affected tenant member or support-session actor. There is no self-approval, threshold bypass or automatic “goodwill” credit.

Recommended Phase 1 request window is 90 days after settlement, further bounded by the verified provider/contract window. This is a commercial policy recommendation, not a statutory entitlement and requires approval.

Removing a licence, cancellation, unused allowance, AI failure handled by release, or delayed payment creates no automatic refund. A service credit may reduce a future invoice only after approval; it never increases an entitlement by itself.

## 13. Decision C12 — refund execution and entitlement disposition

**Recommended policy:** Approval does not mean provider refund success.

Refund lifecycle:

- `PROPOSED`;
- `PENDING_APPROVAL`;
- `APPROVED`;
- `PROVIDER_PENDING`;
- `SETTLED`;
- `REJECTED`;
- `CANCELLED`;
- `REVIEW_REQUIRED`.

After approval, provider refund is invoked outside SQL with exact effect idempotency if supported. Ambiguous outcome is queried/reconciled before retry. Only authenticated authoritative provider status may mark `SETTLED`.

Provider-unavailable or unsupported refunds remain approved-but-pending/review-required; Bidora does not silently record cash/manual completion.

The approved proposal must choose one exact entitlement disposition:

- `NO_ENTITLEMENT_CHANGE` — financial correction not affecting service;
- `REDUCE_ADDITIONAL_LICENCES_AT_RENEWAL` — never evicts current members/invitations mid-term;
- `END_PAID_ACCESS_ON_REFUND_SETTLEMENT` — only for an exact full-term refund, transitioning to retained read-only after settlement;
- `REVIEW_REQUIRED` — where active usage or conflicting payments prevent a safe automatic effect.

Refund settlement atomically records provider effect, immutable credit note, invoice refunded total/state, payment refund state, entitlement disposition, audit and outbox facts. Partial refund never silently terminates the base subscription. Full refund cannot claim deletion or waive retained financial evidence.

VAT reversal uses the same original snapshotted tax/arithmetic profile and credit-note lines. It is not recalculated under the current rate.

## 14. Decision C13 — financial reconciliation and administrative repair

**Recommended policy:** Reconciliation reads provider state and repairs projections only through source-defined transitions; it cannot edit provider references, amounts or immutable documents.

A Billing Administrator with `platform.payment.reconcile` and recent TOTP may request reconciliation for one exact payment/refund or a bounded server-selected due batch. Manual status selection is prohibited.

Workers use database-time UUID leases, bounded retries, provider-specific status-query limits and exact attempt fencing. Cancellation leaves active work for lease expiry. Ambiguous effects remain pending/review-required.

Conflicts requiring human review include amount/currency mismatch, reference reuse, merchant mismatch, unsupported terminal sequence, payment against void invoice, refund exceeding balance, provider settlement without known operation and entitlement effect disagreement.

Resolution never fabricates settlement. An approved financial correction appends adjustment evidence and preserves original observations. Routine direct SQL is not a supported repair path.

## 15. Decision C14 — HTTP contract groups

**Recommended policy:** OpenAPI may be drafted after C1–C15 approval. Provider-specific callback schemas remain blocked until C7 evidence exists.

### Tenant commercial operations

- `POST /v1/tenderpreneur/commercial/licence-quotes` — exact additional-capacity quote.
- `POST /v1/tenderpreneur/commercial/invoices` — issue/reuse invoice for an exact plan/renewal/licence purchase.
- `GET /v1/tenderpreneur/commercial/invoices` and `GET /v1/tenderpreneur/commercial/invoices/{invoice_id}`.
- `GET /v1/tenderpreneur/commercial/receipts/{receipt_id}` and credit-note reads.
- `POST /v1/tenderpreneur/commercial/invoices/{invoice_id}/payments` — initiate/reuse PesaPal operation.
- `GET /v1/tenderpreneur/commercial/payments/{payment_id}` — safe reconciled state.
- `GET /v1/tenderpreneur/commercial/payments/{payment_id}/return` — optional same-origin navigation status endpoint; no mutation/authority.
- `POST /v1/tenderpreneur/commercial/billing-reviews` — request human review, not refund authority.

Writes require exact Origin/CSRF, tenant/context selectors, UUID idempotency, expected versions and strict bounded JSON. Reads are private/no-store. Amounts, provider status and entitlement flags are server-produced.

### Provider callback

- `POST /v1/integrations/pesapal/ipn/{registration_id}` — fixed deployment-registered endpoint; no browser session/CSRF; bounded raw request verification/dedup and durable acceptance only under C7.

A callback response acknowledges durable intake, not payment/entitlement success. Query parameters and fields are accepted only if required by the verified contracted API.

### Platform Billing Administration

- invoice/payment/refund read queues with safe filters;
- exact reconcile request;
- credit/refund proposal;
- exact Platform Owner approve/reject operation;
- refund execution/reconciliation status.

Platform operations accept no tenant database/cell selector. Every mutation has idempotency, optimistic version, reason, recent assurance, audit and safe response semantics.

## 16. Decision C15 — safe errors, evidence and data handling

**Recommended policy:** Stable failures include:

- `commercial_request_invalid`;
- `commercial_not_authorized`;
- `commercial_tax_profile_unavailable`;
- `commercial_quote_stale`;
- `commercial_invoice_not_found`;
- `commercial_invoice_state_conflict`;
- `commercial_payment_in_progress`;
- `commercial_payment_reconciliation_required`;
- `commercial_payment_amount_conflict`;
- `commercial_provider_unavailable`;
- `commercial_refund_not_permitted`;
- `commercial_adjustment_approval_required`;
- `commercial_version_conflict`;
- `commercial_idempotency_conflict`;
- `commercial_dependency_unavailable`.

Provider errors are mapped and redacted. Responses/logs/audit/outbox omit credentials, callback secrets, payment instrument/mobile details, full provider payloads, billing addresses and unnecessary tax identifiers.

Financial evidence may include opaque invoice/payment/refund IDs and numbers, tenant ID, exact minor-unit totals/currency, state/version, provider-reference hash or restricted redacted suffix where operationally necessary, actor IDs, tax/calculation/policy versions and timestamps. Provider raw evidence, if contractually required, is encrypted/restricted and referenced—not copied into general audit or events.

Retention follows the separately approved financial-record policy and active holds. Refund/deletion requests cannot erase required financial evidence.

## 17. Implementation consequences after approval

Approval would authorize a bounded source-design continuation for:

- immutable tax-profile and financial document schema proposals without an active rate seed;
- invoice/payment/refund domain/application contracts and provider-neutral ports;
- exact number/counter, proration and arithmetic implementation;
- strict non-provider-specific OpenAPI drafts from C14;
- source-defined maker/checker financial changes;
- an explicit safer reversal access mode rather than current full `LAST_RECONCILED` access.

Approval would not authorize PesaPal adapter code, callback parsing/authentication, provider settings activation, a tax rate, merchant credentials, migrations, route mounting, payment/refund execution, frontend or production use.

## 18. Deferred verification and acceptance

Later evidence must cover:

- independent finance/tax approval of C1–C4 and actual active tax profile;
- invoice-number concurrency, rollback, void/replacement and year boundary;
- exact proration boundaries, month-end anchors, overflow and rounding;
- duplicate tenant initiation and one provider operation/invoice;
- verified PesaPal sandbox create/query/IPN/refund behavior and authenticity;
- forged/replayed/out-of-order callback and browser-return denial;
- timeout before/after provider effect and query-before-retry;
- settlement amount/currency/reference mismatch and concurrent reconciliation;
- atomic invoice/payment/receipt/subscription/entitlement/audit/outbox effects;
- reversal/read-only behavior and no stale full-access continuation;
- credit/refund maker/checker, bounds, ambiguity and entitlement disposition;
- cross-tenant reads, redaction, retention/hold and restore behavior;
- security review, finance/Tenderpreneur UAT and formal acceptance.

## 19. Requested decision

Approve or amend this bounded set:

- **C1:** VAT-exclusive catalogue plus immutable reviewed tax profiles; no default/rate approval.
- **C2:** exact KES minor-unit `KES_COMMERCIAL_V1` arithmetic and half-up tax rounding.
- **C3:** exact monthly/annual additional-licence proration, 15-minute quotes and no automatic removal refund.
- **C4:** immutable invoices/credit notes/receipts with proposed Nairobi-year document numbers.
- **C5:** minimized billing snapshots and tenant/platform read authority.
- **C6:** one invoice/logical payment, server-owned amount and idempotent initiation.
- **C7:** PesaPal adapter remains blocked until exact contracted API/authenticity evidence.
- **C8:** explicit payment/ambiguity/replay state behavior; browser return and IPN are not settlement.
- **C9:** authoritative exact settlement atomically activates one entitlement effect.
- **C10:** renewal/grace rules and fail-closed retained-read-only reversal handling.
- **C11:** every credit/refund uses Billing Administrator maker plus distinct Platform Owner checker; proposed 90-day request window.
- **C12:** refund completion requires authoritative provider settlement and explicit entitlement disposition.
- **C13:** reconciliation queries provider and never accepts manual fabricated status.
- **C14:** bounded tenant, callback and Platform Billing API groups.
- **C15:** stable safe errors and minimized financial/provider evidence.

## 20. Approval and bounded implementation record

The requesting user explicitly approved C1–C15 and authorized provider-neutral source implementation. BID-API-011C1 now implements only the first bounded operation: issuing one immutable server-priced KES invoice against an exact subscription/quote/billing snapshot and ACTIVE reviewed tax profile.

Control migration `00022_commercial_financial_documents.sql` advances source compatibility to 22 and defines immutable tax profiles, transactional document-number heads, invoices/lines, provider-neutral PesaPal payment observations, receipts, credit notes, maker/checker financial changes, refund operations and hash-only command evidence. It seeds no tax rate or provider configuration. Composite subscription/invoice tenant constraints defend control-plane isolation. Financial document and observation facts are immutable or transition-fenced.

`internal/commercial/domain/financial.go` implements checked KES arithmetic, half-up tax calculation, exact proration, canonical quote/billing/line hashes and Nairobi-year document-number formatting. `internal/commercial/application/invoice.go` prepares the quote outside SQL, then reauthorizes and atomically locks the subscription, active tax profile and invoice number before committing invoice/lines, command, audit and outbox evidence. `internal/commercial/persistence/financial_store.go` is the pgx/sqlc adapter. The quote/billing provider remains an abstract owner port, so the service is unconfigured and unmounted. `api/openapi/drafts/commercial-financial-documents.json` defines the strict invoice boundary and explicitly reports no payment or entitlement effect.

No ACTIVE tax profile exists, so issuance fails closed. No handler, route, PesaPal adapter/callback, payment/refund service, migration execution, provider call, database startup or frontend was added. Generated sqlc artifacts are source output, not functional evidence. At BID-API-011C1, settlement, receipt, refund and platform financial-governance operations remained schema-prepared but application-unimplemented; the later bounded settlement increment is recorded below.

## 21. BID-API-011C2 payment-operation implementation record

The requesting user authorized the provider-neutral payment-operation and query-before-retry slice while explicitly excluding a PesaPal adapter and runtime activation. Control migration `00023_commercial_payment_reconciliation.sql` advances source compatibility to 23. It adds immutable create-attempt identity, mutable reconciliation coordination, database-time two-minute UUID leases, immutable claim/outcome evidence, a maximum schema safety ceiling of nine claims, and transition-protected payment facts. Exact retry count/delays and create-uncertainty delay are mandatory validated configuration facts with no fallback defaults; provider-specific values remain blocked pending C7 evidence.

`internal/commercial/domain/payment.go`, `internal/commercial/application/payment.go` and `internal/commercial/persistence/payment_store.go` implement durable `PROVIDER_CREATE_PENDING` intent before provider dispatch, one logical nonterminal operation per invoice, tenant-scoped replay/read behavior, allowlisted HTTPS redirect validation, and authoritative status-query reconciliation through an abstract provider port. Provider errors and malformed create results remain ambiguous and due for query rather than becoming failed or being blindly recreated. Query outcomes append immutable observations: absence may fail only an unresolved create; pending schedules a configured retry; terminal failure/cancellation completes without entitlement; settled/reversed evidence is held as `ACCOUNTING_COMPLETION_REQUIRED`; amount, currency, reference or transition conflicts are held as `INTEGRITY_CONFLICT`. Cancellation leaves a live lease for expiry recovery. A separately supplied execution identity owns worker audit provenance.

The source deliberately cannot mark a payment `SETTLED`, issue a receipt or activate an entitlement. No callback/IPN contract, PesaPal request/signature/status mapping, provider configuration persistence, handler, route, worker loop, migration execution or provider call is active. `api/openapi/drafts/commercial-payment-operations.json` exposes only the unmounted strict tenant initiation/read contract; a payment may report paid/activated only after the exact atomic settlement effect exists.

## 22. BID-API-011C3 atomic settlement and entitlement record

The requesting user authorized the provider-neutral atomic settlement, receipt and exact entitlement-effect slice while excluding the PesaPal adapter, migration execution and runtime activation. Control migration `00024_commercial_settlement_entitlements.sql` advances source compatibility to 24. It binds every newly issued invoice to one immutable entitlement snapshot and exact expected subscription version; links the authoritative `SETTLED` reconciliation outcome to its immutable observation; adds immutable additional-licence grants and one immutable settlement effect per payment/invoice; and links paid subscription terms to their settlement payment.

`internal/commercial/domain/settlement.go`, `internal/commercial/application/settlement.go` and `internal/commercial/persistence/settlement_store.go` implement one control transaction that locks the exact payment, invoice and subscription; verifies the linked authoritative observation, KES amount/currency, outstanding invoice total, entitlement snapshot and subscription version; allocates a Nairobi-year receipt number; and commits payment `SETTLED`, invoice `PAID`, immutable receipt, paid plan term/usage period or additional-licence grant, subscription version/effect, audit and outbox. Plan activation also preserves the one-lifetime Trial marker, including paid activation without a Trial. Matching completion replays the immutable effect. Effective licence capacity is read from the current term plus only currently effective immutable grants.

The operation fails closed for stale subscription versions, future/not-current terms, unsupported states or unsafely different-plan changes, mismatched amount/currency/reference evidence, missing accounting linkage or duplicate effects. It does not call a provider or interpret provider statuses. No PesaPal adapter, callback/IPN, public settlement route, worker loop, migration execution, receipt download/renderer, refund or runtime composition was added. Functional transaction/race/rollback and entitlement-enforcement integration evidence remain deferred.

## BID-API-011C4 — governed credit-note and refund intent

BID-API-011C4 is `IMPLEMENTED_NOT_VERIFIED` and unmounted. Control migration `00025_commercial_adjustment_governance.sql` advances source compatibility to 25 and requires migration-time reconciliation if any pre-governance financial change exists. A transaction-bound Billing Administrator with fresh AAL2/TOTP may propose one exact adjustment against a settled invoice; any membership in the affected tenant, a support-session context, stale platform context, stale invoice/payment version, exhausted credit balance or refund request beyond the mandatory explicit provider-contract window (capped at 90 days) fails closed. The request records the original tax profile/hash and an exact net/tax allocation under the invoice's approved arithmetic basis without rewriting the original invoice or receipt.

A distinct transaction-bound Platform Owner with fresh AAL2/TOTP may approve or reject within 30 minutes. Approval allocates an immutable Nairobi-year credit-note number and commits the decision, credit note, hash-only idempotency record, audit and outbox together. Refund approval additionally records only an `APPROVED` provider-neutral refund intent and moves the payment to `REFUND_PENDING`; it does not call or query PesaPal, claim cash settlement, apply an entitlement disposition, or alter the original receipt. Rejection creates no credit/refund effect. Source-defined replay returns the immutable adjustment result and conflicting replay fails closed.

The unmounted boundary consists of `internal/commercial/domain/adjustment.go`, `internal/commercial/application/adjustment.go`, `internal/commercial/persistence/adjustment_store.go`, the Platform Administration authority owner adapter, `queries/commercial/adjustment.sql`, and `api/openapi/drafts/commercial-adjustments.json`. No handler, route, worker, provider adapter/call, migration execution, database startup, test implementation/execution, frontend or runtime composition was added. Goose validation, sqlc v1.31.1 zero-drift generation, formatting, module verification, full build, compile-only test discovery and the 132-input checksum verification passed; no test body ran. Functional PostgreSQL transaction/race/rollback verification, exact PesaPal refund behavior and entitlement-disposition policy/execution remain deferred.

## BID-API-011C5 — authoritative payment reversal and retained-read-only hold

BID-API-011C5 is `IMPLEMENTED_NOT_VERIFIED` and unmounted. Control migration `00026_commercial_payment_reversals.sql` advances source compatibility to 26. A transaction-bound Billing Administrator with current `platform.payment.reconcile`, fresh AAL2/TOTP, current platform context, no support-session context and no affected-tenant membership may schedule an exact `SETTLED` payment for provider re-query. The command accepts no provider status, tenant/cell/database selector or amount; it binds the locked payment/version, bounded reason, immutable authority hash and UUID idempotency evidence. It only creates a due reconciliation request for the existing separately configured query-before-retry boundary. A matching authoritative `SETTLED` recheck records `SETTLEMENT_CONFIRMED` and leaves the already-settled payment and entitlement unchanged.

A reversal is authoritative only when that server query produces an exact immutable `STATUS_QUERY` observation linked through an `ACCOUNTING_REQUIRED` outcome, normalized as `REVERSED`, with the original KES amount and currency and an existing immutable settlement effect. For a previously settled payment, reconciliation and reversal handling now commit in one control transaction: the observation/outcome, payment `REVERSED`, invoice `REVIEW_REQUIRED`, subscription `SUSPENDED`, immutable reversal effect, active `RETAINED_READ_ONLY` access hold, audit and outbox facts either all commit or all roll back. The original receipt, invoice facts, settlement observation, term, usage and licence-grant evidence remain unchanged. A replay returns the immutable reversal effect.

The active hold is fail-closed in Commercial reads: subscription summaries project `SUSPENDED`/retained-read-only, new AI reservations and licence-capacity reads fail closed, and new settlement cannot reactivate a subscription while the hold exists. Existing accounting settlement/release completion can still close already-created usage work. Hold release, recovery, financial correction, credit/refund processing and any entitlement restoration remain separately governed and are not implemented. Provider-reported reversal against a payment without an exact prior settlement is treated as an integrity conflict rather than an entitlement transition.

The source boundary consists of `internal/commercial/domain/reversal.go`, `internal/commercial/application/reversal.go`, `internal/commercial/persistence/reversal_store.go`, `queries/commercial/reversal.sql`, the updated reconciliation/domain boundary and control evidence adapter. No PesaPal adapter/callback/query implementation, provider refund, cash movement, receipt rewrite, hold release, migration execution, handler, route, worker, runtime composition, test implementation/execution or frontend was added. Goose validation, sqlc v1.31.1 zero-drift generation, formatting, module verification, full build, compile-only test discovery and the 134-input checksum verification passed; no test body ran. PostgreSQL transaction/race/rollback evidence and downstream enforcement coverage remain deferred.

## BID-API-011C6 — bounded immutable financial-record reads

BID-API-011C6 is `IMPLEMENTED_NOT_VERIFIED` and unmounted at control source compatibility 26. Current tenant membership/context and `tenderpreneur.organization.subscription.read` authorize exact same-tenant receipt and credit-note reads. The read models expose immutable document/payment/invoice/change identifiers, Nairobi-issued document number, allowlisted state/reason, exact KES minor-unit amounts, issue time and content hash only; they do not render a document or disclose billing snapshots, provider references, payment instruments, provider payloads, authority hashes or administrator decision details.

Transaction-authorized Billing Administrators with current AAL2/TOTP, current platform context, no support-session context and no affected-tenant membership may read an exact financial adjustment under `platform.invoice.read` role authority or an exact reversal under `platform.payment.read` role authority. Callers supply only the control-plane target ID and correlation context—never a tenant, cell or database selector. Adjustment reads expose governed maker/checker disposition and linked credit/refund state while omitting authority/request hashes. Reversal reads expose the exact observation/effect linkage, version transitions and active retained-read-only hold while omitting raw provider evidence, provider status codes and references. Every successful platform read commits a minimized audit record in the same control transaction; it emits no business event.

The boundary consists of `internal/commercial/application/financial_read.go`, `internal/commercial/persistence/financial_read_store.go`, `queries/commercial/financial_read.sql`, safe `CreditNote`/existing `Receipt` domain DTOs, the Commercial facade and an audit-only control evidence adapter. No document renderer/download, OpenAPI route, HTTP handler, migration, migration execution, provider call, worker, runtime composition, test implementation/execution or frontend was added. Goose validation, sqlc v1.31.1 zero-drift generation, formatting, module verification, full build, compile-only test discovery and the 135-input checksum verification passed; no test body ran. Functional authorization/isolation/audit and PostgreSQL corruption/race evidence remain deferred.

## BID-API-011C7 — bounded tenant commercial history listings

BID-API-011C7 is `IMPLEMENTED_NOT_VERIFIED` and unmounted at control source compatibility 26. Current same-tenant membership/context and `tenderpreneur.organization.subscription.read` authorize three independently bounded newest-first listings: immutable invoice summaries, redacted payment summaries and a combined financial history. Every operation requires an explicit limit from 1 through 100 and uses stable keyset ordering rather than offsets. Application cursors bind the last UTC instant and UUID; combined-history cursors additionally bind the allowlisted item kind. HTTP cursor serialization/signing remains excluded.

Invoice summaries expose only commercial identifiers, document number/state/version, approved plan/cadence/catalogue purchase facts, exact KES subtotal/tax/total/outstanding amounts and issue/due instants. Payment summaries expose no merchant/provider/configuration reference, redirect, status code or reconciliation reason; they return only payment/invoice IDs, safe product state/version, exact KES amount, timestamps and a derived reconciliation-pending flag. The combined history is a tenant-filtered union of invoice, payment, receipt, credit-note, approved refund-intent and authoritative reversal facts, deterministically ordered by `(occurred_at, kind, id)` and returning only opaque IDs, optional document number, safe state, exact KES amount and occurrence instant.

The source boundary consists of `internal/commercial/application/financial_list.go`, `internal/commercial/persistence/financial_list_store.go`, `queries/commercial/financial_list.sql`, the Commercial facade and generated sqlc artifacts. It adds no Platform billing queue, arbitrary filters/search, route, HTTP cursor, migration, migration execution, provider adapter/call, worker, runtime composition, test implementation/execution or frontend. Snapshot consistency across separate pages is not claimed; immutable financial facts remain stable while mutable payment projection state may advance between requests. Goose validation, sqlc v1.31.1 zero-drift generation, formatting, module verification, full build, compile-only test discovery and the 136-input checksum verification passed; no test body ran. Functional pagination, concurrent-insert, authorization/isolation and query-plan evidence remain deferred.

## BID-API-011C8 — audited Platform Billing financial queues

BID-API-011C8 is `IMPLEMENTED_NOT_VERIFIED` and unmounted at control source compatibility 26. A transaction-bound global authority check permits only a current Billing Administrator with current platform context, `AAL2:TOTP`, no support-session context, no tenant membership/pending-invitation affiliation and the exact `platform.invoice.read` or `platform.payment.read` capability to list the corresponding control-plane queue. Queue requests accept no tenant, cell, database, provider, merchant operation, external reference or arbitrary sort selector.

Invoice, payment, adjustment and reversal queues each require an explicit limit from 1 through 100 and use stable descending `(occurred_at, id)` keyset ordering. The only filters are source allowlists: invoice state; payment state plus reconciliation state; adjustment state plus action; and derived reversal queue state (`PENDING`, `CONFIRMED`, `HELD`, `APPLIED`). Invoice and adjustment queues require `platform.invoice.read`; payment and reversal queues require `platform.payment.read`. Output DTOs expose bounded control identifiers, tenant IDs for operational correlation, governed state/version, exact KES amounts and relevant timestamps. They omit provider references/status codes/payloads, merchant operation IDs, configuration IDs, authority/request hashes, free-text proposal/decision/request reasons, payment instruments and database/routing metadata.

Every successful page read, including an empty result, appends a global control audit row in the same transaction before disclosure. The audit stores the queue name, allowlisted filters, returned count and whether another page exists; it emits no outbox event and records no result rows or private content. Failure to resolve authority, query the queue or append the audit fails closed without returning the page.

The boundary consists of `internal/commercial/application/platform_financial_queue.go`, `internal/commercial/persistence/platform_financial_queue_store.go`, `queries/commercial/platform_financial_queue.sql`, the global financial-authority owner adapter, control audit evidence, the Commercial facade and generated sqlc artifacts. It adds no route, HTTP cursor, schema migration, migration execution, provider adapter/call, callback, worker, runtime composition, test implementation/execution or frontend. Goose validation, sqlc v1.31.1 zero-drift generation, formatting, module verification, full build, compile-only test discovery and the 137-input checksum verification passed; no test body ran. Functional global-affiliation denial, authorization, audit atomicity, pagination/concurrent-insert behavior and PostgreSQL query-plan evidence remain deferred.

## BID-API-011C9 — unmounted Commercial HTTP boundary

BID-API-011C9 is `IMPLEMENTED_NOT_VERIFIED` and unmounted at control source compatibility 26. Two separately registerable Fiber boundaries expose only the already approved browser operations. The Tenderpreneur group supports invoice issue/exact read/list, payment initiation/exact settlement-aware read/list, exact receipt and credit-note reads and combined financial history. The Platform group supports maker/checker adjustment proposal/decision, authoritative settled-payment recheck requests, audited exact adjustment/reversal reads and audited invoice/payment/adjustment/reversal queues. Registration functions are not called by `pilot-api` or another runtime.

Every route uses the existing authenticated exact-Origin browser middleware; unsafe methods additionally require session-bound CSRF. Tenant routes require exact tenant/context selectors which remain non-authoritative and are rechecked by application services. Platform routes require the platform-context generation and accept no tenant, cell, database, provider state, amount, merchant operation or routing selector. UUID idempotency is mandatory for writes, strict JSON rejects duplicate/unknown/missing fields and non-JSON media types, request bodies are bounded, reads reject bodies, query names and queue filters are allowlisted, and pages use bounded opaque keyset cursors. Responses are private/no-store and map only stable safe errors.

The handler returns settlement-aware payment facts only from an exact tenant-scoped payment/receipt/settlement-effect join. `paid`, receipt ID and entitlement effect cannot be inferred from a redirect, callback or provider status. Adjustment proposal and decision now derive the affected tenant server-side from the immutable invoice/change target inside the control transaction; browser input cannot select it. The source-fixed queue action allowlist uses the persisted `ISSUE_CREDIT` and `REQUEST_REFUND` values. Provider references/payloads, hashes used for authority or idempotency, billing snapshots, payment instruments, database/routing metadata and provider diagnostics remain omitted.

`CompletePaymentSettlement` and `CompletePaymentReversal` deliberately have no browser route: they remain server-owned consumers of exact authoritative observations. Likewise no callback/IPN, renderer/download or provider adapter is introduced. The boundary consists of `internal/httpapi/commercial_financial.go`, `internal/httpapi/commercial_platform_financial.go`, `api/openapi/drafts/commercial-financial-reads.json`, additive reviewed query/application changes for server-derived adjustment tenancy and settlement-aware payment reads, and generated sqlc artifacts. No route registration, migration, migration execution, runtime composition, provider call, test implementation/execution or frontend was added. Goose validation, sqlc v1.31.1 zero-drift generation, formatting, module verification, full build, compile-only test discovery, OpenAPI JSON parsing, unmounted-definition inspection and the 137-input checksum verification passed; no test body ran. Functional binding, CSRF, authorization, concealment, pagination, transaction and HTTP-contract evidence remains deferred.

## BID-TEN-004F — paid-write and reversal-hold enforcement

BID-TEN-004F is `IMPLEMENTED_NOT_VERIFIED` and unmounted. Commercial now exposes a current paid-write guard that reads the authoritative control-plane subscription projection and permits mutation only while the effective subscription has `FULL` access within its exact Trial, active-term or grace boundary. Any active immutable reversal hold is projected as `SUSPENDED`/`RETAINED_READ_ONLY` and therefore denies. Payment-pending, suspended, ended, generic review/last-reconciled and unavailable subscription states also fail closed. The guard creates no release, exception, restoration or state transition.

Opportunity integration provides separately composable guarded authorization adapters for manual capture and existing tenant paid-feature mutations: external verification, workspace creation, Bid/No-Bid decisions, requirement creation/assignment, tender-document upload, response/proposal writes, review assignment, approval, external-action/submission evidence, outcome reporting, archive and background document/AI processing. Existing read operations, readiness reads, document-job reads and ownership-continuity administration remain readable/administrative and are not converted into paid writes. The wrapper first resolves normal current membership/capability authority, then checks Commercial access, so the subscription guard is never an authority source and cannot make a tenant selector authoritative.

The already-composed Commercial AI reservation verification now additionally locks/rechecks the exact current tenant usage/subscription row before object or provider access. The query already excludes active reversal holds; verification also binds the current subscription and usage-period IDs to the immutable reservation and reruns the exact AI access-period policy. A hold or other paid-access loss after reservation therefore prevents provider invocation. The Commercial adapter maps that exact denial to the document processor, which atomically cancels the analysis and requests the existing durable release/accounting path instead of retrying provider work indefinitely. It never settles or restores access.

The source boundary consists of `internal/commercial/application/paid_write_access.go`, the Commercial facade, the guarded adapters in `internal/opportunity/integration/paid_write_authorization.go`, and the strengthened AI reservation check. It adds no migration, route, runtime composition/activation, hold release, entitlement restoration, provider call, test implementation/execution or frontend. Goose validation, sqlc v1.31.1 zero-drift generation, formatting, module verification, full build, compile-only test discovery and the 137-input checksum verification passed; no test body ran. Because control authorization and tenant mutation are intentionally separate planes, this source does not claim distributed atomicity: each entry rechecks current control access before routing, while reversal/write race and crash behavior remain deferred to PostgreSQL/cross-plane verification and reconciliation evidence.

## BID-API-011C10 — reversal recovery decision contract

`commercial-reversal-recovery-decision-contract.md` records RR1–RR15 as **APPROVED POLICY — IMPLEMENTATION NOT AUTHORIZED**. RR1–RR15 separate authoritative financial disposition from entitlement disposition; retain every original invoice, receipt, observation, settlement/reversal effect, term and grant; require Billing Administrator maker/distinct Platform Owner checker authority; and keep the exact reversal hold active through generation-fenced cross-plane reconciliation. Restoration can recover only the original effect's unexpired remainder, cannot reset usage or extend a term, and cannot override another hold. Confirmed additional-licence reversal uses an append-only exclusion rather than changing the grant; confirmed base-plan reversal remains retained read-only absent exact qualifying financial evidence.

The approval does not assume that PesaPal supports reversal rescission and does not select replacement-invoice, debit-note or tax treatment. `ORIGINAL_FUNDS_REINSTATED` remains unavailable pending separately approved provider evidence; `REPLACEMENT_SETTLEMENT` remains unavailable pending separately approved finance/tax evidence. The user explicitly directed not to implement RR1–RR15 yet, including the policy-defined `REVERSAL_CONFIRMED` path. No schema, migration, code, query, API/event schema, provider call, runtime activation, test or frontend is authorized or added.

## BE-SIM-03 — proposed credit-application and refund-follow-up contract (not approved for activation)

This is a forward design for C11–C13, **not** a tax treatment, a statutory refund entitlement, a provider-capability assertion or a change to the dated implementation evidence above. Current source issues a credit note linked to the *original* paid invoice. It does not allocate that note to another invoice or reduce that invoice's outstanding balance. A refund approval writes `refund_operations.state=APPROVED`, marks its payment `REFUND_PENDING` and records `credit_notes.state=REFUND_PENDING`; it does not call a provider, settle money or apply an entitlement disposition. Neither operation may be called a successful refund. The payment reconciliation step is callable explicitly but the serving loop has **no payment worker**; do not start one merely to test this contract.

### Credit application — contract to validate before implementation

1. An approved `ISSUE_CREDIT` note is **not** itself a wallet balance, a payment receipt, a cash refund or authority to change an immutable invoice. Proposed eligible value is derived only from an exact same-tenant, KES, non-void, non-refund credit note and immutable prior application facts. `REQUEST_REFUND` notes, ambiguous refund status, reversal holds or disputed credits cannot be silently converted to future-invoice credit. No tenant-submitted credit amount, source invoice, tax profile or routing selector is authoritative.
2. A proposed append-only allocation record would link one source credit-note ID, the originating adjustment/invoice, destination invoice/issue operation, exact gross/net/tax minor units, original tax-profile reference/hash, approved destination tax treatment reference, actor/correlation, UTC instant, versions and idempotency/effect hash. A same-control-transaction lock/fence must enforce `sum(allocated) + sum(refunded or reserved for refund) <= issued credit` and one consumption per allocation key, including concurrent invoice issue and replay. A destination invoice must snapshot each source allocation and allowlisted `APPROVED_CREDIT` line with its own immutable calculation evidence; no retroactive mutation to either issued document or receipt. Repricing, cross-tenant/currency transfer, negative totals, duplicate tax relief and fabricated settlement fail closed.
3. The current invoice constructor computes tax on the entire quote, sets credit to zero and emits only purchase/VAT lines; do not insert an `APPROVED_CREDIT` line or make a zero-payable invoice `PAID` by changing a UI field. A finance/tax-reviewed rule must first specify whether a cross-period service credit reduces taxable base, gross payable, or has a different treatment; original versus destination tax profile, partial allocations/expiry, refunds versus future credit, rounding, and zero-payable entitlement authority all need explicit decisions. Until then, invoice issue and payment initiation continue without applying credit, and the UI must not display an available/spendable balance. Credit application must not activate paid access without a separately proven accounting effect.

### Refund follow-up — contract to validate before provider work

1. Keep `APPROVED` intent immutable and visible with its credit-note/payment linkage. No refund API or worker has been established in the current PesaPal adapter. Require verified merchant/provider API product, sandbox contract, refund eligibility/window (at most the source 90-day cap), exact effect idempotency, authenticated status-query semantics, callback authenticity, amount/currency/merchant mapping, rate limits and recovery evidence before designing an execution port. Finance/legal must approve the production commercial policy independently; neither the source cap nor a browser confirmation grants that approval. Both runtimes currently hard-disable refund proposals and approvals; the approval-time cutoff additionally checks the immutable settlement effect for any later positive policy.
2. An eventual worker would commit a durable intent/claim/lease and immutable attempt *before* any bounded provider call outside SQL; replay and lease expiry do not authorize a second send. If provider acceptance is ambiguous, query authoritative status first. Without an idempotent refund API **and** an authoritative way to resolve ambiguity, hold the operation for human review without sending again. Unsupported/unavailable providers leave approved requests pending or review-required; no synthetic `SETTLED`, offline cash recording or direct SQL repair.
3. A provider-observed terminal refund would require exact payment/reference, merchant, KES amount, settlement and credit-note linkage, deduplicated immutable observations and one version-fenced control transaction for refund/payment/invoice/credit-note states, original-tax accounting, approved entitlement disposition, audit and outbox. Partial refunds cannot end full-term access; full refunds cannot erase original settlement, receipt or usage. Reversal/access holds remain independent; no tenant database join or cross-plane restoration is implied. Keep client receipts explicit: `APPROVED` and `PROVIDER_PENDING` are **not** cash returned; only an authoritative, persisted terminal settlement may say so.

**Gates / next implementation task:** obtain named finance/tax/legal decisions on credit allocation and production refund eligibility, plus verified provider-contract evidence; then draft the exact append-only schema, owner-module ports and stable API/outbox contracts with replay/concurrency tests. No schema, migration, API, worker, tax rule, provider send, database connection or production-policy activation is added by this design note. A disposable database must be expressly designated before functional API and BID-MVP-05 persistence verification.
