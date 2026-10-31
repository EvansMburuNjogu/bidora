# Manual-capture database foundation

## Scope/status

User request on 2026-10-18: complete database-design execution and backend foundations. Implemented the bounded database slice required by the already approved **unverified manual external-tender capture** operation.

Status: **IMPLEMENTED_NOT_DATABASE_VERIFIED**. The [Option A table inventory](../schema/option-a-tables.json) records **53** required/control-or-tenant tables with implementation/gate status; inventory is not blanket schema approval. Migration/query files are executable and Goose/sqlc parser/generator checks pass, but no PostgreSQL instance was started or modified. Real constraints, transactions, grants, migration execution, two-database isolation and recovery remain unverified. This is not completion of all Bidora database tickets.

Excluded because their policy/contracts remain open: identity recovery/MFA, broader grant-management and last-admin schemas (the later approved fixed capture-contributor invitation table is recorded below), tenant provisioning execution, documents, verified-source transitions, bid workspaces/decisions/requirements, approvals, packages, submissions/outcomes, procurement/exchange, AI/payment schemas and production topology. No web work, server provisioning, role/database creation or real data.

## Implemented files

- `migrations/control/00001_tenant_registry.sql`: provider-neutral Option A tenant registry, database-cluster/assignment routing, schema-version state and control compatibility metadata. No credential/provisioning tables or role/database creation.
- `migrations/control/00002_audit_outbox.sql`: append-only plane-local control audit and immutable outbox facts with optional tenant/actor provenance; delivery fields remain operational. Advances control schema to version 2.
- `migrations/control/00003_password_identities.sql`, `queries/identity/password.sql` and generated `internal/identity/persistence/sqlc/`: selected Go-owned email/password foundation with unique application-supplied lookup identity and Argon2id-v19 PHC-only credentials. Advances control schema to version 3; no sessions, challenges, MFA, recovery or membership authority.
- `migrations/control/00004_email_identity_policy.sql` and `internal/identity/email/`: confirmed ASCII mailbox policy, 254-byte database safeguards, full-address lowercase lookup and no alias rewriting. Advances control schema to version 4; existing hypothetical rows remain explicitly gated on reconciliation/constraint validation.
- `migrations/control/00005_email_verification_challenges.sql`, `queries/identity/verification.sql` and `internal/identity/verification/`: SHA-256-only 32-byte opaque verification tokens, 30-minute expiry, one active challenge, resend invalidation and atomic single-use user verification. Advances control schema to version 5; no raw token, provider, route or automatic metadata deletion.
- `migrations/control/00006_browser_sessions.sql`, `queries/identity/session.sql` and `internal/identity/session/`: domain-separated SHA-256 session/CSRF hashes, 30-minute idle/12-hour absolute limits, rotation linkage, current user/auth-epoch refresh guards and revocation primitives. Advances control schema to version 6; raw tokens/cookies and HTTP policy are not persisted.
- `migrations/control/00007_tenant_membership_capabilities.sql`, `queries/authorization/membership.sql` and generated `internal/authorization/persistence/sqlc/`: current membership state, immutable/revocable capability effects, generation bumps and fail-closed external-capture authorization with active DENY precedence. Advances control schema to version 7; no role catalog, grant handler or platform-admin bypass.
- `migrations/control/00008_google_external_identities.sql` and `queries/identity/google.sql`: stable Google-subject identity bindings and takeover-safe signup/sign-in/linking persistence. Advances control schema to version 8.
- `migrations/control/00009_tenant_invitations.sql`, `queries/authorization/invitation.sql` and generated authorization persistence: approved fixed capture-contributor invitations with 72-hour SHA-256 token evidence, one-active tenant/email uniqueness, atomic matching-email acceptance, audit and outbox. Advances control schema to version 9; no administrator mutation or last-admin policy.
- `migrations/control/00010_password_recovery.sql`, `queries/identity/recovery.sql` and generated identity persistence: approved password-user recovery core with SHA-256-at-rest 32-byte tokens, 30-minute expiry, one active challenge, resend invalidation, atomic consumption, credential/auth-epoch advancement and all-session revocation. Advances control schema to version 10; public routes are conditionally composed with approved identity 3/hour and network 10/hour Redis limits, 15-minute denial and explicit SMTP configuration.
- `queries/control/registry.sql` and generated `internal/tenancy/persistence/sqlc/`: fetch control compatibility and an ACTIVE Tenderpreneur assignment only when cluster, assignment generation and both tenant schema sets are compatible. Callers must already hold validated membership; tenant ID alone is never authorization.

- `migrations/tenant_common/00001_command_evidence.sql`: tenant-local append-only audit, durable outbox with immutable fact fields, and capture idempotency records with approved no-automatic-TTL semantics.
- `migrations/tenant_common/00002_schema_compatibility.sql`: bounded application read/write compatibility intervals for the common set.
- `migrations/tenant_common/00003_organization_access_projection.sql`: one organization profile per tenant DB and local identity/membership projections for ownership/display. Projection state is deliberately not authorization.
- `migrations/tenant_common/00004_event_inbox.sql` and generated `internal/eventing/persistence/sqlc/`: immutable payload-free `(consumer,event_id)` receipts for same-transaction database-effect deduplication; no automatic expiry. Common schema advances to version 4.
- `migrations/tenant_common/00005_outbox_actor_provenance.sql`: additive actor provenance for new tenant outbox facts, protected as immutable and advancing common schema to version 5. The `NOT VALID` check still rejects new null actors while allowing a separately reviewed reconciliation for any hypothetical pre-v5 rows before dispatch.
- `migrations/tenant_tenderpreneur/00001_unverified_opportunity_capture.sql`: opportunity root and immutable unverified source versions only; composite FK guarantees current source belongs to its opportunity.
- `migrations/tenant_tenderpreneur/00002_schema_compatibility.sql`: ordered extension compatibility row, requiring common version 2.
- `queries/opportunity/capture.sql`: parameterized, schema-qualified steps for idempotency claim/lock/completion, root/source insert, current-source binding, audit and outbox.
- `internal/opportunity/persistence/capture_store.go`: tenant-lease-bound unit-of-work adapter over bounded pgx transactions and generated queries. It maps nullable capture fields, locks/replays idempotency, checks affected rows, and writes ID-only audit/outbox evidence with actor provenance; no broker dispatch occurs.
- `queries/opportunity/compatibility.sql` and `internal/opportunity/persistence/compatibility.go`: typed fail-closed verification that both sets support application generation 1 for reads and writes. This does not select/authorize a tenant or establish readiness by itself.
- `sqlc.yaml` and generated `internal/opportunity/persistence/sqlc/`: pgx/v5 typed code and transaction-compatible `DBTX`/`WithTx`; generated artifacts must not be hand-edited.
- `migrations/checksums.json`: reviewed pre-release SHA-256 inputs. Released migrations will be immutable; later changes require additive files.
- `migrations/README.md`: set ordering, distinct Goose ledgers, role/recovery boundaries and required live evidence.
- `tests/database/migrations_test.go`: centralized checksum, scope, migration-safety, invariant, query and generator-provenance regression checks.

## Complete Option A inventory and schema start

`schema/option-a-tables.json` is the bounded schema plan: 20 control-plane and 33 Tenderpreneur-tenant tables. Current statuses are 23 implemented-not-database-verified, 3 planned dependencies and 27 policy/identity/document blocked. Every entry has a database/schema/table, implementation status and exact gate. Procurement tenant, exchange and AI/search databases are excluded from Option A and listed as future-release databases. Duplicate/empty/unknown-status regression tests guard the inventory.

Executable schema now starts in three sets: control registry, tenant common and Tenderpreneur opportunity. The control registry stores secret/endpoint **references**, never credentials, and cannot create databases/roles. Active routing requires ACTIVE tenant/cluster, one active assignment matching the tenant assignment generation, tenant-common version >=5, Tenderpreneur version >=2, and COMPATIBLE schema rows matching that generation. Membership authorization remains a separate mandatory control-plane check before this query.

## Design resolution

This executable slice refines the earlier proposed denormalized opportunity row for capture:

- `opportunities` is a mutable root with version/current-source pointer and actor/timestamps.
- `source_versions` is immutable and owns captured facts, source references, explicit unknowns, interpreted UTC instant plus source zone/text, `UNVERIFIED` state and canonical request hash.
- Nullable source fields preserve unknown facts. `closes_at` requires nonblank source zone/text; omission retains ambiguous raw zone/text without inventing an instant.
- Source document UUID is intentionally a logical reference until BID-DB-005 creates the local document schema/FK. A route accepting non-null document IDs remains disabled until document-owner access/eligibility and FK tests exist.
- Distinct idempotency keys do not title-deduplicate. Duplicate resolution and workspace uniqueness remain later approved operations.
- IDs/timestamps are application-generated; no PostgreSQL extension or hidden default tenant.

The capture transaction must claim/lock scoped idempotency, insert root/source, bind current source, append audit/outbox, and complete safe replay metadata in one tenant transaction. Generated SQL enables these steps but does not itself orchestrate or authorize them. On replay, current identity/context/reference authorization is still mandatory. SQL failure must roll back every step.

## Tool review

Pinned build-time tools are invoked explicitly and were not added to application `go.mod`:

| Tool | Pin and integrity | Inspected license |
|---|---|---|
| sqlc | v1.31.1; module `h1:+V+BjBJfFNPX/RFfL8eiZD9jk9lVJUEGGllWvnYNqbc=`; commit `a95e91d70ad9e1181253c333a1cfdd75ae4b95a5` | MIT, Riza, Inc. 2024 |
| Goose | v3.29.0; module `h1:dtEnBCT7CqkHIUCdgrXsrmeisaN8gFkRU4Y5OEwif4o=`; commit `8b457ba3778983bec89012cd3bf6ab07a61ac0e4` | MIT, Staskawicz/Vitek/Fridman notices |

Module checksum verification is Go proxy/module-sum evidence, not independent signed-release verification. Tool execution downloaded tool-only dependency graphs into the Go module cache; they are not runtime dependencies or project pins. Linker warned that temporary tool object files targeted macOS 13 while linking on macOS 12; both tools executed successfully, but this is not a supported-host certification.

## Verification actually executed

Go 1.27.2, darwin/amd64:

```sh
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir migrations/control validate
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir migrations/tenant_common validate
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir migrations/tenant_tenderpreneur validate
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 diff
GOPROXY=off go test -count=1 -race ./...
GOPROXY=off go vet ./...
GOPROXY=off go build ./...
go mod verify
```

All passed; sqlc diff emitted no drift. During the compatibility continuation, the first unchanged Goose validation attempt failed before tool execution because `sum.golang.org` reset the connection while verifying tool-only `github.com/paulmach/orb@v0.13.0`. Checksum verification was not disabled; the identical pinned command was retried and passed. Goose `validate` verifies migration-file structure, while sqlc parsing/generation adds PostgreSQL grammar/type analysis; neither executes PostgreSQL semantics.

Linux/amd64 active backend build and centralized opportunity/database test binaries cross-compiled with `CGO_ENABLED=0`; Linux runtime was not run. Scripted DB-interface tests now execute the capture generated methods without PostgreSQL; combined opportunity domain/application/persistence/transport coverage is **85.0%**. This does not prove SQL, transaction, trigger or constraint behavior. Goose/sqlc checks and the full race/vet/build/module suite were rerun after the identity migration and generated package. Linux/amd64 build plus database-test compilation passed without execution. Cached govulncheck v1.9.0 built/run with verified Go 1.27.2 after the identity addition reported zero reachable/imported findings; existing module-only GO-2026-5932 for unused openpgp remains. Build tools and their tool-only dependency graphs were not covered by that application scan.

## Other-table decision

The policy-neutral tenant additions now include singleton organization/user projections and an append-only payload-free event inbox. Inbox receipt and database effects must commit in one transaction; it does not make email/object-store effects exactly once, authorize event provenance fields or define cleanup. No automatic inbox expiry is implemented.

The next two policy-neutral tenant tables—singleton organization profile and local user profiles—are now implemented. They preserve registry/identity/membership references and projection versions but contain no role/capability grants; stale `ACTIVE` text never authorizes a request.

The next apparent table dependency—documents/document versions for optional `source_document_version_id`—was reviewed and **not implemented**. BID-DB-005/BID-API-004/BID-ARC-008 still require approved format, scan-state, URL, retention/hold and storage policies. A partial reference table would falsely imply that bytes are clean/immutable/available. The logical UUID remains blocked for non-null runtime use. Bid/workflow tables are likewise unnecessary for unverified capture and retain unresolved transition policy. This is deliberate anti-diversion/default-deny behavior, not forgotten schema work.

Schema compatibility metadata was added instead because it is policy-neutral and required to refuse incompatible databases. Goose ledgers remain distinct and authoritative for migration history; compatibility rows express only binary read/write intervals.

## Remaining blockers before an operational endpoint

1. A separately authorized TLS-capable PostgreSQL test instance. The existing isolated minimal native build lacks TLS and cannot validate the server-suitable adapter without weakening it.
2. Actual clean/upgrade/concurrent migration, constraint, grant, rollback, transaction, query-plan, restore and two-tenant tests listed in `migrations/README.md`.
3. Deployment-bound migrator/runtime role names and grants; runtime must not own/DDL schemas or mutate audit/source history.
4. The document schema/FK and access port before non-null source-document references are allowed.
5. Approved identity/session/context/CSRF policy and database routing; no request header may select a database or confer authority.
6. Event contract acceptance and application transaction orchestration. Generated queries alone do not publish or execute business effects.

No route is registered and API readiness still does not claim database availability. Recovery is forward-fix or restore into a new target; Down SQL is development-only for owned empty databases and was not executed.

## Fixed invitation continuation — 2026-10-18

After explicit policy approval, control migration 9 was applied to the isolated TLS PostgreSQL fixture through its migrator role. Generated sqlc code and checksums were refreshed. Live issuance and acceptance using the migrator role prove deny-first inviter authority, resend invalidation with one active invitation, fixed capture-only invitee authority, verified matching-email and active-tenant guards, atomic issue/accept audit/outbox persistence, generic replay denial and exactly one winner under concurrent consumption. This is database/application evidence, not runtime-route evidence: runtime grants for the new table and membership mutation, SMTP invitation delivery, HTTP Origin/CSRF binding and pilot composition remain blocked. The earlier historical no-database statements above describe their original increments and are not reinterpreted as current fixture state.
