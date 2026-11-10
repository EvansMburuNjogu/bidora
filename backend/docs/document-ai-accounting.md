# Document AI reservation recheck and completion accounting

> Historical implementation evidence: the unmounted `completionworker` package was removed in BE-SIM-01; the application completion runner remains.

Status: **IMPLEMENTED_NOT_VERIFIED — UNMOUNTED**

Date: 2026-10-20

Tickets: BID-AI-003, BID-EVT-003, BID-TEN-004E

## Outcome

The tender-document provider path now has source-level accounting gates around OpenAI execution:

1. claim only an exact `CLEAN`, measured, entitlement-linked job with a tenant checkpoint in `RESERVED`;
2. recheck current processing authority;
3. recheck the exact control reservation against checkpoint, tenant, requester, usage kind, assignment generation and measured credit amount;
4. invoke the provider only through the new entitled processor constructor;
5. atomically commit successful cited suggestions and a durable tenant `SETTLE_PENDING` request using the extraction run as execution ID;
6. reconcile that request through Commercial settlement, then atomically mark the tenant checkpoint `SETTLED`;
7. for authorization loss detected before provider invocation, atomically cancel the run/job and create `RELEASE_PENDING`, then reconcile Commercial release and tenant `RELEASED`.

The legacy constructor has no reservation verifier. Its analysis path now releases the claim back to `READY_FOR_PROVIDER` and returns unavailable before object access or provider invocation. Scan behavior is unchanged.

## Durable completion protocol

Tenant-common migration `00007_ai_entitlement_completion.sql` advances source compatibility from 6 to 7 and adds immutable completion requests and acknowledgements. Existing checkpoint states from migration 6 are used as follows:

- `RESERVED` → `SETTLE_PENDING` → `SETTLED`;
- `RESERVED` → `RELEASE_PENDING` → `RELEASED`.

Each request stores the exact checkpoint, reservation, execution, operation, correlation and—only for settlement—positive actual credits. Each local transition commits checkpoint version, immutable evidence, audit and tenant outbox in one tenant transaction.

Document charging is deterministic: successful PDF/DOCX analysis settles the same credit amount calculated from the immutable measurement and reserved before provider execution. Provider token counts do not alter customer credits.

## Cross-plane reconciliation

`DocumentCompletionCoordinator` now recovers or claims one durable pending tenant request through the approved [completion reconciliation policy](document-completion-reconciliation-loop-decision.md), validates tenant/generation and current placement, invokes idempotent Commercial settlement/release outside the tenant transaction, rechecks placement, then records the tenant acknowledgement in a fenced transaction. It remains unmounted.

A crash after control commit but before tenant acknowledgement is safe: the same reservation, execution ID, operation and actual credits replay at Commercial, after which tenant acknowledgement is retried. Commercial terminal replay now verifies the immutable transition's execution ID, terminal status and actual credits rather than accepting status alone.

No distributed SQL transaction is used. The durable tenant request remains the reconciliation source until local acknowledgement commits.

## Fail-closed provider boundary

The exact reservation check occurs after current-authority revalidation and before object retrieval/OpenAI. It does not grant authority; it confirms that the prior Commercial reservation still applies to the claimed immutable measurement.

Provider or transport errors after invocation follow approved Option 1 in the [OpenAI ambiguous outcome decision](openai-ambiguous-outcome-decision.md). Unknown post-dispatch errors default to `AMBIGUOUS_EFFECT`: the run/job fails with stable `PROVIDER_OUTCOME_UNKNOWN`, no suggestions are persisted, and deterministic measured credits enter `SETTLE_PENDING` without automatic retry. A proven `CONFIRMED_NO_EFFECT` instead enters `RELEASE_PENDING`. Runtime activation remains blocked by the independent parser, migration, composition and verification gates.

Archive eligibility now treats `RESERVED`, `SETTLE_PENDING` and `RELEASE_PENDING` document checkpoints as pending work even when the analysis job itself is terminal. This prevents an unresolved post-provider reservation from being hidden by a failed job.

## Source changes

- tenant-common migrations `00007_ai_entitlement_completion.sql` and `00008_ai_entitlement_completion_reconciliation.sql`;
- `internal/opportunity/application/document_completion_runner.go` and the unmounted `internal/opportunity/completionworker/` boundary;
- completion request/acknowledgement application and persistence in `internal/entitlementcheckpoint/`;
- exact Commercial reservation verification and execution-fenced terminal replay;
- `internal/opportunity/application/document_ai_accounting.go`;
- `internal/opportunity/integration/document_ai_accounting.go`;
- `internal/opportunity/persistence/document_completion_store.go`;
- entitled provider construction and atomic extraction/settlement-request persistence;
- archive accounting-pending guards;
- regenerated sqlc artifacts and checksum manifest.

## Explicit exclusions

- runtime worker mounting, deployment composition and startup;
- functional/race verification of the fenced runtime worker/service-loop policy;
- parser or OpenAI activation;
- automatic retry after an ambiguous provider/transport outcome;
- provider billing reconciliation;
- migration execution or service startup;
- functional, race, rollback, provider or database tests.
