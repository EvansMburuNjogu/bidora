# Generation-aware document entitlement acknowledgement

Status: **IMPLEMENTED_NOT_VERIFIED — UNMOUNTED**

Date: 2026-10-20

Tickets: BID-EVT-003, BID-AI-003, BID-TEN-004E

## Outcome

A tenant-routed control acknowledgement can now atomically update both the durable tenant entitlement checkpoint and the exact measured tender-analysis job. This closes the source-level gap between control `RESERVED` evidence and provider readiness without treating event routing metadata as authority.

The unmounted server-owned source dispatcher is now recorded in [ai-entitlement-acknowledgement-dispatch.md](ai-entitlement-acknowledgement-dispatch.md). Runtime worker-loop composition, broker delivery, provider execution and reservation settlement/release are not included.

## Atomic application

`DocumentEntitlementAcknowledgementService` is constructed for one server-routed tenant and exact assignment generation. It rejects an acknowledgement whose tenant or generation does not match that bound context before opening a transaction.

Inside one tenant-database transaction it:

1. validates and consumes the fixed control acknowledgement through `entitlementcheckpoint/application.AcknowledgeWithinTransaction`;
2. records the inbox receipt, immutable checkpoint transition, checkpoint audit and checkpoint outbox evidence;
3. locks the job reached only through the immutable checkpoint → measurement link;
4. verifies the job's snapshotted assignment generation and requester against the checkpoint;
5. applies exactly one job disposition;
6. appends Opportunity-owned audit and outbox evidence.

Any failure rolls back both owner effects. The outer pgx adapter binds each owner's application transaction port to the same `postgres.DBTX`; no handler or application service calls another module's generated queries.

## Dispositions

- `RESERVED` requires the checkpoint's exact reservation ID and moves only `PENDING_RESERVATION` with `AI_ENTITLEMENT_PENDING` to `READY_FOR_PROVIDER`.
- `DENIED` requires one stable `LIMIT_EXCEEDED`, `ACCESS_UNAVAILABLE` or `SUBSCRIPTION_UNAVAILABLE` code and moves the exact job to terminal `FAILED`.

Reserved application emits `ai.document.entitlement_reserved.v1`; denied application emits `ai.document.entitlement_denied.v1`. Payloads contain identifiers, disposition and stable denial code only—no document text, object locator, prompt, provider response or credential.

The original requester remains audit actor provenance; neither that fact, the event tenant ID nor assignment generation independently grants authority. Provider execution still performs its existing current-authority recheck.

## Replay and races

- A replay whose checkpoint and job already have the corresponding terminal disposition returns as replay without duplicate Opportunity evidence.
- A checkpoint acknowledged earlier while its job remained pending can be reconciled by replaying the same disposition; the job effect is then committed atomically.
- Opposite disposition, changed reservation/denial facts, wrong tenant/generation, missing measurement link or an unrelated job state conflicts and rolls back.
- Acknowledgement cannot make an unmeasured or unlinked job ready.
- Provider claiming still requires `READY_FOR_PROVIDER`, exact measurement/link, `CLEAN` version and checkpoint state `RESERVED`.

## Source changes

- Exported transaction-scoped acknowledgement logic from the Entitlement owner without exposing its repository.
- Added an Entitlement persistence transaction binder returning only the owner application `Tx` interface.
- Added Opportunity acknowledgement application policy and same-database composition store.
- Added `queries/opportunity/document_entitlement_acknowledgement.sql` and generated sqlc artifacts.
- Advanced the reviewed checksum manifest to 119 inputs; no migration version changed because migration 29 already defines the required states and relations.

## Verification performed

- sqlc v1.31.1 generation and drift inspection.
- `gofmt` on affected Go files.
- `GOTOOLCHAIN=go1.27.2 go build ./...`.
- Goose migration validation and checksum verification are recorded in the slice report.

No test body, database transaction, event delivery, parser, service, migration or provider was executed. This is `IMPLEMENTED_NOT_VERIFIED`.

## Remaining work

- runtime service-loop composition and bounded per-attempt deadline policy;
- approved stale-assignment/cell-movement reconciliation and poison-event policy;
- provider-time reservation/applicability recheck;
- runtime composition of durable completion reconciliation;
- ambiguous post-invocation provider outcome policy;
- centralized duplicate/opposite/out-of-order/concurrent acknowledgement tests;
- PostgreSQL rollback/race/isolation evidence;
- worker composition and deployment authorization.
