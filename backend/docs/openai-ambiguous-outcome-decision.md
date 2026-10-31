# OpenAI ambiguous outcome decision packet

Status: **OPTION 1 APPROVED — SOURCE IMPLEMENTED_NOT_VERIFIED — RUNTIME UNMOUNTED**

Date: 2026-10-20

Tickets: BID-AI-003, BID-EVT-003, BID-TEN-004E

## Approved decision

On 2026-10-20 the requesting user proceeded with the recommended Option 1. When OpenAI may have received or executed a tender-analysis request but Bidora has no trustworthy valid result, Bidora settles the immutable measured credits, creates no suggestions, records `PROVIDER_OUTCOME_UNKNOWN` and does not retry automatically.

This decision applies only after the current-authority and exact-reservation checks pass and provider invocation begins. It does not alter pre-provider failures, successful cited output, human verification, provider configuration or model selection.

## Existing fixed outcomes

These outcomes do not require a new decision:

- Before provider invocation: terminal authorization denial creates `RELEASE_PENDING` and releases the reservation through durable reconciliation.
- Successful strict response with usable cited suggestions: commit suggestions and `SETTLE_PENDING` for the deterministic measured credits.
- Local object/open/configuration failure before any provider request can be sent: no provider effect occurred; release is permissible once the implementation can prove that classification.

The unresolved case begins when request bytes may have crossed the provider boundary.

## Why the result is ambiguous

The current adapter uses a synchronous `POST https://api.openai.com/v1/responses` with `store:false`. A timeout, connection loss during upload/response, truncated 2xx body, invalid 2xx body or local cancellation may occur after OpenAI received some or all input. Bidora may lack both a valid result and a provider response ID.

Because remote response storage is disabled, Bidora cannot assume the response can later be retrieved. Retrying the document blindly could create another provider effect. A transport error alone cannot prove that nothing was processed or billed.

## Required classification

Regardless of the accounting choice, implementation must classify outcomes as:

1. `CONFIRMED_SUCCESS`: a bounded, strict, valid response was received and persisted.
2. `CONFIRMED_NO_EFFECT`: Bidora has evidence that no provider request was sent or the provider definitively rejected it before execution under an accepted provider contract.
3. `AMBIGUOUS_EFFECT`: request execution cannot be ruled out, including timeout/cancellation after dispatch, interrupted upload/response, malformed/truncated success response, and provider statuses not contractually proven no-effect.

Unknown errors default to `AMBIGUOUS_EFFECT`. Raw transport/provider diagnostics remain private and are not tenant-visible, logged in ordinary telemetry or placed in audit/outbox payloads.

## Options

### Option 1 — settle ambiguous attempts (recommended for pilot)

- Settle the already reserved deterministic document credits.
- Persist no AI suggestions because no valid result exists.
- Mark the analysis terminal with stable reason `PROVIDER_OUTCOME_UNKNOWN`.
- Do not automatically retry.
- Preserve immutable execution, reservation, measurement and redacted outcome evidence.
- A future explicit retry is a new separately authorized/reserved operation; its limits and user contract must be defined before implementation.

Advantages:

- avoids free repeated provider effects and quota abuse;
- releases the concurrent-job slot deterministically;
- requires no unsupported provider lookup or new support authority;
- preserves one accounting disposition for every possibly executed provider attempt.

Trade-off: a tenant may consume credits without receiving usable suggestions.

### Option 2 — release ambiguous attempts

- Release credits and concurrency.
- Persist `PROVIDER_OUTCOME_UNKNOWN` with no suggestions.
- Do not automatically retry.

Advantages: the tenant is not charged without a usable result.

Trade-offs: Bidora may absorb provider cost, and repeated explicit attempts could externalize provider expense unless a separately approved attempt/rate policy exists.

### Option 3 — hold for manual disposition

- Keep the reservation and concurrent slot held in a reconciliation-required state.
- A separately authorized human later chooses settle or release.

This option is not currently implementable: no maximum hold duration, decision authority, permitted evidence, support-access boundary, escalation SLA or final-owner rule is approved. Indefinite holds violate the requirement for explicit finite limits.

## Approved pilot policy

Option 1 is approved with these exact safeguards:

- `CONFIRMED_NO_EFFECT` releases; `CONFIRMED_SUCCESS` settles; `AMBIGUOUS_EFFECT` settles.
- Actual customer credits for document analysis equal the immutable pre-provider measured credits; token counts do not change them.
- No automatic provider retry after `AMBIGUOUS_EFFECT`.
- No suggestions or human-verification candidates are created from incomplete/invalid output.
- The job records only stable `PROVIDER_OUTCOME_UNKNOWN`; private wrapped diagnostics are not persisted as business evidence.
- Archive may proceed only after the settlement checkpoint reaches `SETTLED` and immutable ambiguity evidence exists.
- Provider billing reconciliation remains operational evidence and never rewrites tenant credit history.
- Any future retry endpoint requires a new operation ID, fresh current authority/applicability checks and a new reservation under a separately approved attempt policy.

## Not decided here

- Whether OpenAI contractually guarantees no effect for specific HTTP statuses.
- Whether an idempotency header is supported for this exact Responses API operation and retained consistently with `store:false`.
- Provider invoice/billing dispute handling.
- Customer-facing credit refunds or goodwill adjustments.
- Support/admin authority to override accounting.
- Automatic retry counts, backoff or user-triggered retry contract.

The approved policy is now represented in source, but provider execution remains unmounted pending parser/Linux isolation, migration, runtime composition and verification gates.
