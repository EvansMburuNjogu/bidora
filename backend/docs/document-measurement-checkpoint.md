# Persisted document measurement and entitlement orchestration

Status: **IMPLEMENTED_NOT_VERIFIED — UNMOUNTED**

Date: 2026-10-20

Tickets: BID-AI-003, BID-EVT-003, BID-TEN-004E

## Outcome

Tender analysis now has a source-defined fail-closed path from an exact immutable `CLEAN` document version to immutable measurement evidence, Commercial credit calculation and a durable tenant-to-control entitlement reservation request. OpenAI cannot be claimed from the ordinary `QUEUED` state.

This source is not composed into a running worker. No migration was executed and no provider was invoked.

## Schema and state machine

Tenderpreneur migration `00029_document_measurement_entitlement.sql` advances Phase 1 source compatibility from 28 to 29 and adds:

- immutable `ai.document_measurements`, one per analysis job and document version;
- immutable `ai.document_measurement_entitlements`, linking a measurement to its durable tenant-local entitlement checkpoint;
- analysis states `MEASURING`, `PENDING_RESERVATION` and `READY_FOR_PROVIDER`;
- exact blocked reason `AI_ENTITLEMENT_PENDING` only while pending reservation.

A measurement snapshots exact media type, byte size, SHA-256, parser/semantics/profile versions, measured unit/value, applicability hash, usage kind, estimated credits, requester, assignment generation and correlation. It stores no extracted document text, object key or relationship target.

Tenderpreneur migration `00030_document_measurement_leases.sql` advances source compatibility to 30 and adds the approved five-minute UUID-fenced lease, separate three-attempt counter, due time, terminal measurement code and immutable claim/outcome evidence. Rollback fails if measurement/link/attempt evidence or active measurement lease/retry facts remain.

## Application sequence

`DocumentMeasurementCoordinator.ProcessNext` performs one bounded operation:

1. Reconcile the oldest measured job whose entitlement link was interrupted, or claim one `QUEUED` clean analysis job as `MEASURING`.
2. Recheck current document-processing authority and exact assignment generation.
3. Open only the server-owned exact object/version and run the approved process measurer outside SQL.
4. Require exact byte/hash agreement and validate the returned applicability hash.
5. Calculate document credits through the Commercial owner interface:
   - PDF: one credit per measured page.
   - DOCX: one credit per started 3,000 extracted characters.
6. Atomically insert immutable measurement, audit and `ai.document.measured.v1` outbox evidence and move the job to `PENDING_RESERVATION`.
7. Recheck authority again.
8. Request the existing durable entitlement checkpoint using the measurement ID as the source operation and the analysis job as the target.
9. Atomically validate and link the exact pending checkpoint to the measurement with audit evidence.

The parser/object call is never inside a SQL transaction.

## Crash and replay behavior

Measurement commit and entitlement request deliberately use separate durable owner transactions:

- A crash before measurement commit leaves the job under its five-minute fenced lease. Recovery appends `LEASE_EXPIRED`, invalidates the old token and schedules 30-second/120-second backoff or terminates the third attempt as `MEASUREMENT_RETRY_EXHAUSTED`.
- Parser/object/credit failure releases `MEASURING` back to `QUEUED` without reserving credits.
- A crash after measurement commit leaves immutable evidence and `PENDING_RESERVATION`.
- The next coordinator pass finds the unlinked measurement first and replays the entitlement request with the same source operation.
- The checkpoint service returns the existing checkpoint only when its request hash matches.
- Link insertion validates tenant, target job, measurement source operation, requester, assignment generation, usage kind and estimated credits in one transaction. It accepts the checkpoint's durable pending/reserved/denied state so a fast acknowledgement cannot race the provenance link.
- Re-linking the same checkpoint is idempotent; a different checkpoint conflicts.

This avoids pretending that two owner transactions are atomically nested. The oldest unlinked measurement is intentionally reconciled before new measurement work; retry scheduling/backoff remains a later worker policy.

## Provider fail-closed guard

The existing provider claim query no longer claims `QUEUED` analysis jobs. It requires all of:

- job state `READY_FOR_PROVIDER`;
- exact immutable measurement for the same job/document version;
- exact measurement-to-checkpoint link;
- checkpoint state `RESERVED`;
- document still `CLEAN`.

The unmounted generation-aware application in [document-entitlement-acknowledgement.md](document-entitlement-acknowledgement.md) now moves only an exact linked `RESERVED` checkpoint to `READY_FOR_PROVIDER` and makes a stable denial terminal `FAILED` in the same tenant transaction as checkpoint/inbox evidence. The unmounted server-owned source dispatcher is recorded in [ai-entitlement-acknowledgement-dispatch.md](ai-entitlement-acknowledgement-dispatch.md). Provider release returns only to `READY_FOR_PROVIDER`, never to pre-measurement `QUEUED`. No dispatcher service loop or provider worker is composed.

Archive eligibility treats `MEASURING`, `PENDING_RESERVATION` and `READY_FOR_PROVIDER` as pending work.

## Module boundaries

- Opportunity application owns orchestration and measurement evidence.
- Commercial remains the owner of credit rules through `DocumentCreditCalculator`.
- Entitlement Checkpoint remains the owner of durable reservation requests through `DocumentEntitlementRequester`.
- The Opportunity pgx adapter owns only Opportunity/measurement SQL.
- Existing Entitlement application/persistence commits checkpoint, audit and outbox evidence in its own transaction.

No handler or worker calls another module's generated repository directly.

## Generated artifacts

Added/updated reviewed inputs:

- `migrations/tenant_tenderpreneur/00029_document_measurement_entitlement.sql`
- `migrations/tenant_tenderpreneur/00030_document_measurement_leases.sql`
- `queries/opportunity/document_measurement.sql`
- `queries/opportunity/document_processing.sql`
- `queries/opportunity/archive_preparation.sql`
- `queries/opportunity/bid_archive.sql`
- generated Opportunity sqlc artifacts
- `migrations/checksums.json`; the current reviewed manifest contains 124 inputs after the later completion-reconciliation slice

Generated sqlc files were produced by sqlc v1.31.1 and were not manually edited.

## Verification performed

- `gofmt` on affected Go files.
- sqlc v1.31.1 generation.
- `GOTOOLCHAIN=go1.27.2 go build ./...` passed.
- Goose Tenderpreneur migration validation and sqlc drift/checksum checks are recorded with the slice report.

No test body, migration, PostgreSQL transaction, process parser, object-store operation, entitlement event, service or provider was executed. Status remains `IMPLEMENTED_NOT_VERIFIED`.

## Remaining work

- dispatcher service-loop composition plus approved stale-assignment/poison-event reconciliation;
- runtime composition of the entitled provider and completion-reconciliation loops;
- approved handling for ambiguous post-invocation provider/transport outcomes;
- centralized application, persistence, PostgreSQL race/rollback and subprocess tests;
- migration 29 execution and runtime grant/isolation evidence;
- Linux parser sandbox acceptance and final artifact evidence;
- worker composition and deployment authorization.
