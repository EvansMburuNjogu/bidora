# BID-API-011C10 — reversal-hold release, financial correction and entitlement-restoration decision contract

Status: **RR1–RR15 APPROVED — IMPLEMENTATION NOT AUTHORIZED**  
Scope: resolution of an existing authoritative payment-reversal effect and its active `RETAINED_READ_ONLY` subscription hold  
Personas: Billing Administrator maker; distinct Platform Owner checker; server-owned reconciliation executor  
Depends on: approved Commercial C1–C15, implemented-not-verified reversal source at control compatibility 26, exact provider-contract evidence and accountable finance/tax review

RR1–RR15 are approved policy. This approval creates no permission to add a schema, migration, handler, route, provider adapter/call, runtime composition, worker, test or frontend. It does not select PesaPal behavior, accounting treatment or tax treatment that has not been verified by the accountable owners. `ORIGINAL_FUNDS_REINSTATED` and `REPLACEMENT_SETTLEMENT` are explicitly unavailable until their provider and finance/tax evidence is approved.

## 1. Outcome and non-negotiable boundaries

The purpose is to resolve a reversal hold without erasing why Bidora failed closed. Resolution must answer two separate questions:

1. **Financial disposition:** does authoritative evidence prove that the original funds were reinstated, that a separately settled replacement financial effect exists, or that the reversal remains financially effective?
2. **Entitlement disposition:** may the exact original entitlement continue for its remaining immutable interval, must an exact reversed additional-licence effect be excluded, or must paid access remain ended/read-only?

Release of a hold is not itself proof of payment and never grants access. Restoration is permitted only from a source-defined disposition backed by exact immutable financial evidence. Original payment observations, reconciliation outcomes, receipts, invoices, settlement effects, reversal effects, tax snapshots, terms, licence grants and access-hold activation evidence remain immutable.

Excluded:

- manual selection of provider status, amount, currency or settlement time;
- editing/deleting a reversal, receipt, credit note, invoice, payment observation, paid term or licence grant;
- direct SQL repair as a supported operation;
- extending an expired term, resetting allowance, replacing consumed AI credits or re-running cancelled work;
- treating a credit note, refund approval, callback, browser return, support assertion or Platform role as payment;
- releasing legal, privacy, security, retention or unrelated Commercial holds;
- cash/manual/off-platform payment recording;
- provider-specific status mapping or reversal-rescission assumptions without C7 evidence;
- debit-note/accounting implementation before finance/tax approval.

## 2. RR1 — one append-only recovery case per reversal effect

**Approved policy:** Recovery begins from one exact immutable `payment_reversal_effect`, not from a tenant-selected invoice or provider reference. One active recovery case may exist per reversal effect. It binds:

- reversal effect, access hold, payment, invoice, subscription and tenant IDs;
- settlement and reversal observation IDs and hashes;
- settlement/reversal effect hashes;
- original entitlement effect kind (`PLAN_ACTIVATION` or `ADDITIONAL_LICENCES`);
- exact original term/grant interval and amount/currency;
- expected payment, invoice, subscription and hold versions/states;
- server-observed current assignment generation for later reconciliation;
- creation actor, assurance, correlation, reason code and immutable request hash.

A repeated matching command replays the same case. A changed request under the same idempotency key conflicts. A new case is prohibited while another case for that reversal remains nonterminal. A terminal case is never reopened; new evidence requires a linked successor case.

## 3. RR2 — authority and segregation of duties

**Approved policy:** A transaction-bound Billing Administrator may propose a recovery disposition only with:

- current global Platform context;
- exact `platform.payment.reconcile` capability;
- `AAL2:TOTP` established no more than 15 minutes earlier;
- no support-session context;
- no active membership or pending invitation in the affected tenant;
- current administrator/catalogue version.

A distinct transaction-bound Platform Owner with exact `platform.governance.approve` and fresh `AAL2:TOTP` must approve or reject. Maker and checker cannot be the same person and neither can have an affected-tenant tie or support context. Platform Owner status is not wildcard authority. Tenant administrators and controlled-support users may view only already-authorized redacted tenant or operational status; they cannot propose, approve, execute or release.

The proposal expires after 30 minutes. Expiry creates no effect and requires a new proposal against current versions and evidence. Approval records intent; it does not by itself release the hold or restore access.

## 4. RR3 — accepted financial bases

**Approved policy:** A proposal must select exactly one source-defined financial basis:

### `ORIGINAL_FUNDS_REINSTATED`

An authenticated authoritative server query, under a verified provider contract, proves that the original reversal was rescinded and the exact original KES amount is again finally held for the same immutable merchant/payment operation. The evidence must be newer than the reversal observation and linked by immutable reconciliation outcome. A callback, provider dashboard screenshot, free text or ordinary `SETTLED` status is insufficient unless the verified provider state machine explicitly proves post-reversal reinstatement.

This basis remains unavailable until provider-contract evidence defines whether such a state exists, its finality and its exact mapping. Source must not guess it.

### `REPLACEMENT_SETTLEMENT`

A distinct payment has an already committed authoritative settlement effect for an approved corrective invoice/charge that binds the exact affected subscription, entitlement effect and remaining obligation. The replacement cannot reuse the reversed payment, fabricate an amount, or silently apply an unrelated overpayment. Corrective invoice/debit-note and tax treatment require accountable finance/tax approval before this basis can be implemented.

### `REVERSAL_CONFIRMED`

The reversal remains authoritative. No payment is treated as available. This basis can close the recovery case while retaining read-only access, or can remove only the reversed additional-licence effect if an independently valid base term remains. It cannot restore a reversed base-plan entitlement.

Ambiguous, pending, partially matched, mismatched, manually asserted or unsupported evidence produces no approvable financial basis.

## 5. RR4 — entitlement dispositions

**Approved policy:** The maker selects exactly one disposition allowed by the financial basis and original settlement effect:

- `RESTORE_ORIGINAL_EFFECT_REMAINDER` — only with `ORIGINAL_FUNDS_REINSTATED` or exact `REPLACEMENT_SETTLEMENT`; restores the original effect only until its immutable original end.
- `EXCLUDE_REVERSED_ADDITIONAL_LICENCES` — only for an original `ADDITIONAL_LICENCES` effect with `REVERSAL_CONFIRMED`; an append-only exclusion makes that grant ineffective without deleting or changing it.
- `END_AFFECTED_PAID_ACCESS` — records that the reversed base-plan effect remains unavailable and leaves the subscription retained read-only/ended according to its immutable term.
- `CONTINUE_RETAINED_READ_ONLY` — closes no financial ambiguity and does not release the active hold; it records reviewed deferral only.

`NO_ENTITLEMENT_CHANGE` on a credit note, refund approval or service credit cannot restore access. There is no “courtesy restore,” grace override, support override, trial restart or unbounded temporary access.

## 6. RR5 — exact restoration calculation

**Approved policy:** Restoration is derived server-side from the original settlement effect and current authoritative lifecycle state.

For `PLAN_ACTIVATION`:

- the plan, cadence, catalogue version, limits, commercial period, usage periods and price snapshot remain exactly those originally settled;
- restoration is impossible at or after the original commercial-period end;
- no suspended/reversed interval extends the term or moves a usage boundary;
- current consumed/reserved Bid, document, storage and AI usage remains consumed/reserved;
- pending cancellation and lawful lifecycle boundaries remain effective;
- restoration produces `ACTIVE/FULL` only if the exact term is currently effective and no other hold or lifecycle blocker exists.

For `ADDITIONAL_LICENCES`:

- restoration or exclusion applies only to the exact immutable grant and original effective interval;
- restoration is impossible at or after that interval's end;
- confirmed reversal creates an append-only exclusion effect; it never reduces capacity below current active-membership/invitation reservations automatically;
- if exclusion would leave excess reservations, new invitations/memberships remain blocked and the case requires ordinary organization remediation—Bidora does not evict people automatically;
- a separately valid, current base term may return to `FULL` after exclusion reconciliation if no other blocker exists.

No disposition resets a lifetime-Trial marker, creates a fresh term, grants replacement AI credits, reopens archived Bids, requeues AI/document work or reverses prior usage.

## 7. RR6 — original financial projections and corrected current state

**Approved policy:** Historical records remain unchanged:

- original payment retains immutable settlement and reversal observations/effects;
- original receipt remains issued and linked;
- original invoice remains historically linked to `REVIEW_REQUIRED` reversal evidence;
- original term/grant remains immutable.

A new append-only recovery decision and completion effect records the current resolution. Reads may derive a safe resolution such as `REINSTATED`, `REPLACED`, `CONFIRMED_REVERSED` or `UNRESOLVED`, but must not relabel historical observations. If a mutable payment/invoice projection is advanced for query efficiency, it must point to the exact recovery effect and cannot erase `REVERSED`/`REVIEW_REQUIRED` history.

Finance/tax owners must approve whether `REPLACEMENT_SETTLEMENT` requires a replacement invoice, debit note or another legally valid document. Until then, Bidora must not claim a corrected receivable, tax liability or cash balance.

## 8. RR7 — staged cross-plane reconciliation

**Approved policy:** Approval commits only an immutable recovery decision and a durable reconciliation request in the control plane. The active reversal hold remains `ACTIVE`, subscription access remains `RETAINED_READ_ONLY`, and paid writes remain denied.

A server-owned reconciler later:

1. locks the exact approved decision and active hold;
2. revalidates all financial links, expected versions, current authority-independent policy and absence of superseding evidence;
3. resolves the tenant's current assignment from the control registry—never from the request;
4. records a generation-fenced control checkpoint;
5. applies or acknowledges the exact tenant-local entitlement projection effect in the currently assigned compatible cell;
6. records an immutable tenant acknowledgement and control completion;
7. only then executes the final control transaction that appends hold-release completion and advances the subscription/current-access projection.

There is no distributed SQL transaction. Until every required finite checkpoint is complete, the hold remains active and access remains read-only. Old-generation acknowledgements are rejected and never forwarded automatically. Assignment movement, unavailable cells, incompatible schemas or ambiguous commits retain the hold and require idempotent reconciliation.

If no tenant-local projection is affected by a future implementation, the approved target set must explicitly say so; the system cannot silently skip reconciliation because a cell is inconvenient.

## 9. RR8 — hold release is append-only and narrowly scoped

**Approved policy:** The immutable activation row is not updated or deleted. Release is represented by an append-only release effect linked one-to-one to the active hold and approved recovery decision. Effective hold state is derived from activation plus a valid terminal release effect.

A release effect includes:

- hold/reversal/recovery decision IDs and hashes;
- exact financial and entitlement disposition;
- maker/checker authority hashes;
- all required checkpoint/acknowledgement hashes and assignment generations;
- prior/resulting subscription versions and resulting access mode;
- completion actor, timestamps, policy version, audit and outbox IDs;
- a canonical effect hash.

It releases only the exact `PAYMENT_REVERSAL` hold. Other active reversal, legal, privacy, retention, security, fraud, lifecycle or integrity holds continue to deny as their own policies require. `FULL` cannot result while any independent blocker remains.

## 10. RR9 — final state matrix

**Approved policy:**

| Financial basis | Original effect | Entitlement disposition | Hold/access result |
|---|---|---|---|
| `ORIGINAL_FUNDS_REINSTATED` | plan or licences | `RESTORE_ORIGINAL_EFFECT_REMAINDER` | Release only after checkpoints; `FULL` only inside original interval and absent other blockers |
| `REPLACEMENT_SETTLEMENT` | plan or licences | `RESTORE_ORIGINAL_EFFECT_REMAINDER` | Same, bound to exact replacement settlement; never extends original interval unless a separately purchased new term says so |
| `REVERSAL_CONFIRMED` | additional licences | `EXCLUDE_REVERSED_ADDITIONAL_LICENCES` | Release after exclusion checkpoints; base subscription may be `FULL` only if independently current/valid |
| `REVERSAL_CONFIRMED` | plan activation | `END_AFFECTED_PAID_ACCESS` | Recovery closes; subscription remains `SUSPENDED`/`ENDED` and retained read-only |
| unresolved/ambiguous | any | `CONTINUE_RETAINED_READ_ONLY` | Hold stays active; no restoration |

Any other combination is invalid. A term that expired during reconciliation cannot return to `FULL`. A newly settled ordinary renewal/new term follows its own approved settlement path and does not retroactively erase this case.

## 11. RR10 — concurrency, idempotency and replay

**Approved policy:** Proposal, decision, checkpoint, acknowledgement and completion each use UUID idempotency, canonical request hashes, expected versions and immutable effect uniqueness.

- proposal locks the reversal, hold, payment, invoice and subscription;
- checker decision rechecks maker/checker separation, authority, expiry and expected versions;
- approval races permit one terminal decision;
- one release effect may exist per hold;
- one exclusion/restoration effect may exist per original settlement effect and recovery case;
- duplicate completion returns the existing exact result;
- changed-body replay, stale version, superseding observation, new active hold or mismatched checkpoint conflicts;
- cancellation does not release a lease or hold early; lease expiry enables recovery;
- crashes before final completion leave access read-only and resume from durable checkpoints.

No retry repeats a provider charge or refund. Any potentially effective external financial action remains governed by query-before-retry in its owning payment operation.

## 12. RR11 — reads and disclosure

**Approved policy:** Tenant financial reads may show only a safe, plain-language resolution status and immutable document links already authorized for that tenant. They omit Platform actors, authority hashes, provider evidence/references, internal reason detail, routing data and checkpoint diagnostics.

Platform Billing exact reads and queues may expose bounded IDs, safe states, expected/current versions, disposition, checkpoint state and timestamps under existing `platform.payment.read`, AAL2 and no-affiliation rules. Every successful Platform read remains audited before disclosure. Requests accept no tenant, cell, database, provider-status or assignment selector.

No read may say “payment restored,” “hold released” or “access restored” before the exact terminal effect exists. `APPROVED` and `PROPAGATING` are pending states, not completion.

## 13. RR12 — evidence, events and safe errors

**Approved policy:** Audit/outbox evidence may contain opaque control IDs, tenant ID, KES amount/currency, allowlisted basis/disposition/state, versions, policy version, effect hashes, actor IDs and timestamps. It must not contain provider payloads/references, payment instruments, billing addresses, tender content, cell/database identifiers, credentials or free-form investigation material.

Approved stable errors for any later authorized source implementation:

- `commercial_reversal_recovery_invalid`;
- `commercial_reversal_recovery_not_found`;
- `commercial_reversal_recovery_not_authorized`;
- `commercial_reversal_recovery_evidence_required`;
- `commercial_reversal_recovery_approval_required`;
- `commercial_reversal_recovery_expired`;
- `commercial_reversal_recovery_conflict`;
- `commercial_reversal_recovery_reconciliation_pending`;
- `commercial_reversal_recovery_unavailable`.

Events are facts, not bearer authority. Approved event family names are `commercial.reversal_recovery.proposed.v1`, `.decided.v1`, `.reconciliation_requested.v1`, `.hold_released.v1` and `.closed_read_only.v1`. Exact event schemas remain a later approval artifact.

## 14. RR13 — correction and refund separation

**Approved policy:** Reversal recovery does not replace the approved credit/refund workflow.

- A credit note corrects the invoiced financial amount under its original tax basis; it does not prove cash movement or restore entitlement.
- A refund intent/settlement returns funds; it does not rescind a reversal or restore paid access.
- A replacement settlement is a new authoritative financial effect, not a credit/refund approval.
- Confirmed reversal of additional licences requires an exclusion effect even if a credit note is also issued.
- Required credit/debit/tax documents must be linked to the recovery case but remain governed by finance/tax-approved document rules.

The system cannot net unrelated invoices, credits, refunds or overpayments to reach an access-restoration amount without a separately approved allocation contract.

## 15. RR14 — blocked implementation and verification gates

RR1–RR15 policy authority is approved, but implementation remains blocked by the user's explicit direction and until all applicable remaining gates clear:

- separate bounded implementation authorization;
- exact PesaPal contracted post-reversal/query semantics for `ORIGINAL_FUNDS_REINSTATED`;
- finance/tax/legal treatment of reversal correction, replacement settlement, credit/debit documents and original receipt/invoice display;
- exact tenant-local projection/checkpoint inventory;
- role-capability catalogue impact, if existing capabilities are insufficient;
- migration and query review;
- safe API/event contracts;
- deterministic domain, PostgreSQL race/rollback and cross-plane crash/replay tests;
- two-tenant isolation and no-affiliation authorization evidence;
- provider sandbox evidence where provider states are involved;
- security review, finance/Tenderpreneur UAT and deployment acceptance.

Required later tests include duplicate approval, stale evidence, expiry, maker/checker collision, affiliation races, superseding observations, multiple holds, term expiry during propagation, assignment movement, old-generation acknowledgement, crash before/after each checkpoint, exclusion below reserved licences, replay and restoration without allowance reset.

## 16. RR15 — recovery and rollback posture

**Approved policy:** Pre-release schema recovery is forward-fix or restore into a new isolated target; released financial migrations are never destructively auto-downed with evidence present. A failed recovery operation is resumed from immutable checkpoints. It never rolls back by deleting evidence or reactivating access.

A restored control/tenant database cannot serve this operation until current assignment, authorization, active holds, recovery decisions and checkpoint generations are reconciled. If evidence is incomplete or inconsistent, the exact hold remains effective and access stays read-only.

## 17. Approval record and continuing gates

The requesting user approved RR1–RR15 as drafted. The approved decisions are:

- **RR1:** one append-only, version-bound recovery case per reversal effect;
- **RR2:** Billing Administrator maker plus distinct Platform Owner checker, fresh TOTP and no tenant/support ties;
- **RR3:** only original-funds reinstatement, exact replacement settlement or confirmed reversal as financial bases;
- **RR4:** four allowlisted entitlement dispositions and no courtesy restore;
- **RR5:** restore only the original remaining interval with no usage reset or extension;
- **RR6:** immutable original evidence plus append-only corrected-current-state facts;
- **RR7:** staged, generation-fenced cross-plane reconciliation while the hold remains active;
- **RR8:** append-only, exact-hold release that cannot override another blocker;
- **RR9:** the approved financial/entitlement final-state matrix;
- **RR10:** exact idempotency, version, race, replay and crash semantics;
- **RR11:** minimized tenant/platform reads and no premature completion claim;
- **RR12:** safe errors and minimized evidence/event families;
- **RR13:** strict separation from credit/refund and no unapproved netting;
- **RR14:** provider, finance/tax, contracts, tests, security and acceptance remain mandatory gates;
- **RR15:** forward-fix/isolated restore and fail-closed restored-state reconciliation.

The user explicitly directed **do not implement yet**. No provider-neutral or provider-specific implementation is authorized by this approval. `ORIGINAL_FUNDS_REINSTATED` remains unavailable until exact contracted provider evidence is separately approved. `REPLACEMENT_SETTLEMENT` remains unavailable until accountable finance/tax approval of its financial-document and tax treatment. `REVERSAL_CONFIRMED` is policy-defined but also remains unimplemented until a later bounded implementation authorization.

This approval does not authorize provider-specific statuses, a replacement-invoice/debit-note tax treatment, schemas, migrations or migration execution, queries, API/event schemas, routes, provider calls, runtime activation, workers, tests, frontend, deployment or release acceptance.
