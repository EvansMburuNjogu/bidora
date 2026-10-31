# Option A backend verification order

Status: **V1–V9 AND EXPANDED VER-02R DATABASE RESILIENCE VERIFIED_TECHNICALLY — V10 BLOCKED AT COMPOSED LIVE-RUNTIME GATE**  
Source boundary: **OPTION A FUNCTIONAL IMPLEMENTATION FINISHED; feature development frozen**  
Tickets: BID-BE-001 through BID-BE-007 are `VERIFIED_TECHNICALLY`.

## Purpose

Convert the frozen Option A backend from source-complete to technically verified in dependency order without mixing new features into evidence work. A failed gate stops dependent verification; fixes must be bounded defects, not scope expansion.

## Execution rules

- Use synthetic data and the existing Go/pgx/PostgreSQL, Redis, MinIO, ClamAV, SMTP and OpenAI boundaries.
- Do not start or edit `web/`; backend/API verification remains the active boundary.
- Apply migrations only through reviewed migrator credentials. Runtime startup never migrates.
- Never print credentials, API keys, raw tokens, MFA secrets, recovery codes, private documents or storage keys.
- Run race/concurrency cases against real dependencies where the acceptance criterion is transactional.
- Preserve database-per-organization isolation and verify both positive and negative tenant cases.
- Record exact commands, versions, pass/fail/skipped evidence and defects. Do not advance after a blocking failure.
- Ticket progression is `IMPLEMENTED_NOT_VERIFIED` → `VERIFIED_TECHNICALLY` → `UAT_PENDING` → `ACCEPTED`; compilation alone cannot advance status.

## Ordered verification packets

### V0 — Frozen-source baseline — VERIFIED_BASELINE 2026-10-18

Confirmed deterministic sqlc generation, migration checksums, OpenAPI/inventory JSON, `gofmt`, build, compile-only discovery and dependency integrity from the frozen source. This detects drift but does not verify functionality.

Recorded environment and results:

- Go `1.27.2`, `darwin/amd64`; module `bidora/backend`; 249 modules and 52 Go packages.
- `go mod verify`: all modules verified.
- `gofmt -l` over 347 Go files under `cmd`, `internal` and `tests`: zero unformatted files.
- sqlc `v1.31.1`: 57 generated Go files had byte-identical SHA-256 manifests before and after generation; zero drift.
- Migration manifest: 88 entries, zero missing files and zero mismatches.
- Goose `v3.29.0 validate`: control, tenant-common and Tenderpreneur sets passed. The Go linker emitted the known macOS deployment-version warning; commands exited successfully.
- JSON parsing: health contract, 41 grouped draft contracts, table inventory and checksum manifest passed.
- `go build ./...`: passed for all 52 packages.
- `go test -run '^$' ./...`: compile-only discovery passed for all 52 packages; no functional test body was selected.
- Frozen inventory: 13 control migrations, 5 tenant-common migrations, 26 Tenderpreneur migrations, 42 SQL query sources, 41 grouped draft contracts and 88 catalog entries.
- Freeze marker occurs exactly once in `backend/docs/application-development.md`.

No migration was applied, no service or provider was contacted, no listener was started and no functional, race, security or UAT evidence was produced. BID-BE-001–007 remain `IMPLEMENTED_NOT_VERIFIED`.

### V1 — Schema, roles and compatibility baseline — VERIFIED_TECHNICALLY 2026-10-21

Apply and verify clean plus supported-upgrade migration paths to control 13, tenant-common 5 and Tenderpreneur 26. Verify dedicated migrator/runtime roles, runtime DDL denial, compatibility fail-closed behavior, rollback-on-failure, two private tenant databases and no caller-selected routing fallback.

Required before every BID-BE ticket.

#### V1a existing-database upgrade — VERIFIED_UPGRADE 2026-10-20

The initial preflight was refused while the endpoint was stopped. After explicit authorization, only the existing Bidora-owned PostgreSQL cluster was started with unchanged configuration; Redis and web remained stopped. The private mode-0600 environment and PostgreSQL 15.19 client connected to loopback `127.0.0.1:55432` using TLS. Migrator DSNs did not carry the required path, so each Goose connection explicitly set the reviewed `bidora_migrations,pg_catalog` search path; migrator roles had USAGE/CREATE there and no CREATE on `public`.

Upgrade results:

- control: ledger/compatibility 9 → 13 through migrations 00010–00013;
- tenant A: common remained 5; Tenderpreneur ledger/compatibility 2 → 26 through migrations 00003–00026;
- tenant B: common remained 5; Tenderpreneur ledger/compatibility 2 → 26 through migrations 00003–00026;
- all five migration-set reruns were clean no-ops;
- migrator checks found `identity.mfa_factors`, `identity.mfa_transactions`, all three session-assurance columns, `bid.archive_events` and `bid.membership_ownership_transfer_events`;
- existing runtime credentials read exact compatibility 13 / 5 / 26 over TLS in control and both tenants.

The existing cluster was stopped after evidence capture. No database, role or service configuration was created or changed, and no Redis/web process or application listener was started. This verifies only the supported existing-data upgrade and exact compatibility; clean-database migration, grants for every new operation, concurrent migration runners, rollback fault injection and restore remain V1 work.

#### V1b clean-database preflight — BLOCKED_FIRST_GATE 2026-10-20

After the user froze all 13 Phase 1 expansion umbrellas and authorized resuming V1 only, the existing Bidora-owned PostgreSQL endpoint was started with unchanged configuration. PostgreSQL 15.19 accepted verified TLS connections on its existing loopback endpoint; Redis and web remained stopped. The mode-0600 environment exposed only the existing control/Tenant A/Tenant B migrator and runtime DSNs plus the CA path.

All six exposed database identities were confirmed non-superuser with `rolcreatedb=false` and `rolcreaterole=false`; TLS was on and `password_encryption` was `scram-sha-256`. This is the correct least-privilege posture for normal migration/runtime work, but none can create the disposable clean databases required by the first remaining V1 criterion. Local socket authentication is explicitly rejected, and no bootstrap/provisioner credential or pre-created empty target is exposed by the approved environment. Reusing the populated databases would destroy V1a evidence and is not an acceptable clean-path test.

The run stopped at this first blocker without applying migrations, changing roles/grants, creating/dropping a database, running tests or advancing to concurrent migration, fault injection, restore or V2. PostgreSQL was stopped again; Redis and web remained stopped throughout.

Smallest unblock options, requiring an explicit deployment/test-fixture decision, are either:

1. provide one bounded bootstrap/provisioner session that may create and later remove three named disposable synthetic databases while preserving the existing role split; or
2. provide three pre-created empty synthetic targets with distinct existing migrator/runtime credentials and reviewed ownership/grants.

Do not grant `CREATEDB`, `CREATEROLE`, owner membership or superuser to the existing migrator/runtime identities. V1 remains incomplete and BID-BE-001–007 remain `IMPLEMENTED_NOT_VERIFIED`.

#### V1c bounded provisioner authorization — STOPPED_NO_CREDENTIAL 2026-10-20

The user authorized one bounded session through the existing bootstrap/provisioner boundary to create exactly three disposable clean-migration databases, with an explicit instruction to stop without changes if no usable bootstrap credential existed. Preflight found no bootstrap/provisioner environment key and no private bootstrap/provisioner credential file under the Bidora-owned local integration boundary. Only the six existing migrator/runtime credentials remain exposed.

The endpoint was already stopped and was not started. No login was attempted, no credential was generated/reset/recovered, and no role, membership, grant or database was changed. Redis and web remained stopped. This preserves the instruction not to elevate existing migrator/runtime identities and confirms that the authorized provisioner session cannot be opened with the available material.

V1 remains blocked until a deployment/test-fixture owner either supplies an existing usable bootstrap credential through an approved private channel or independently pre-creates three empty disposable databases with reviewed owner/migrator/runtime separation. Creating or resetting a bootstrap credential is a distinct security-sensitive action and is not authorized by the failed-session permission. V2 must not start.

#### V1d one-time bootstrap reset — BLOCKED_AUTHENTICATION 2026-10-20

The user separately authorized a one-time reset of the existing synthetic `bidora_local_bootstrap` login through PostgreSQL single-user recovery. With the cluster stopped, the run generated 32 random bytes encoded as a bounded base64url password, stored it only in a mode-0600 temporary file, and submitted an `ALTER ROLE ... LOGIN PASSWORD` command under explicit `scram-sha-256`. Secret-bearing input/output files were mode 0600 and were not printed or committed.

The single-user process exited zero, after which the unchanged cluster started successfully and PostgreSQL reported ready. The mandatory first TLS/SCRAM login as `bidora_local_bootstrap` failed with PostgreSQL's generic password-authentication failure. Because single-user exit status alone did not prove that the SQL command succeeded and its redacted output was removed before this login check, the reset effect is **ambiguous** and is not claimed successful. The run stopped at that first failure. It did not inspect/assume the provisioner, grant owner membership, alter migrator/runtime attributes, create a database, run a migration or start a test.

PostgreSQL was stopped and the temporary password/reset/output files were removed. Redis and web remained stopped. Existing migrator/runtime credentials and databases were not touched; only the already-inaccessible bootstrap credential may have changed. V1 remains blocked and V2 must not start. A future recovery attempt requires a reviewed recovery procedure that validates single-user SQL outcome without exposing the verifier and verifies the reset before deleting diagnostic evidence; do not repeat the same command blindly.

#### V1e corrected bootstrap recovery — BLOCKED_SINGLE_USER_SQL 2026-10-20

The user authorized exactly one corrected recovery attempt and conditionally authorized V2 only after V1 completion. With PostgreSQL stopped, the attempt generated a fresh 32-byte hex credential, wrote credential/SQL/output files with mode 0600, requested SCRAM-SHA-256 plus infinite login validity for the documented bootstrap identity, and added a safe marker query requiring a login-capable SCRAM verifier. The single-user output was retained for the required result check and never printed.

The single-user process returned output classified as containing a PostgreSQL SQL error. Per the stop-first instruction, the attempt ended before starting PostgreSQL or trying TLS/SCRAM authentication. The generated credential and protected SQL/output files were deleted immediately. No provisioner or owner membership was inspected/assumed/changed; no other role attribute, grant, database, migration or test changed. Redis and web remained stopped.

The exact server diagnostic may contain the secret-bearing statement and was deliberately not retained or disclosed after classification, so this evidence does not invent a root cause. Another blind credential-reset attempt is not authorized. V1 remains `BLOCKED_SINGLE_USER_SQL`; although V2 is conditionally authorized, it was not started and cannot start until V1 is verified.

#### V1f read-only role inspection — BLOCKED_ROLE_MODEL_MISMATCH 2026-10-20

The user authorized one read-only single-user inspection of the stopped cluster. The inspection selected only aggregate booleans/counts from `pg_authid` and `pg_auth_members`; it selected no password/verifier and reported no exact role name. Mode-0600 SQL/output files were removed after classification. PostgreSQL, Redis and web were not started.

Findings:

- exactly one superuser exists and its name follows the expected Bidora-owned prefix;
- the historical documented `bidora_local_bootstrap` role does not exist;
- the historical documented `bidora_local_provisioner` role does not exist;
- there is therefore no bootstrap-to-provisioner or provisioner-to-owner membership;
- the three migrators have no direct memberships in the three historical owner roles;
- the three runtime roles have zero provisioner/owner memberships.

This proves the previous reset failed because it targeted a historical role model that is not present in this integration cluster. The retired local-bootstrap draft is not evidence of the live cluster's role topology and must not be executed or treated as its authority. The actual single superuser identity was intentionally redacted; no credential or verifier data was retained.

No `ALTER`, `GRANT`, `REVOKE`, `CREATE` or `DROP` statement ran. V1 is now `BLOCKED_ROLE_MODEL_MISMATCH`, not an unexplained authentication failure. Continuing requires an explicit choice between using the actual Bidora-owned superuser as a bounded synthetic provisioner for disposable databases, or provisioning separate reviewed roles/targets outside this verification run. Do not create the absent historical roles merely to satisfy old fixture documentation. V2 remains unstarted.

#### V1g actual-superuser provisioner attempt — BLOCKED_SINGLE_USER_SQL 2026-10-20

The user selected the actual sole Bidora-prefixed superuser as the bounded synthetic V1 provisioner. The stopped-cluster procedure generated a fresh 32-byte hex credential in mode-0600 files and submitted one guarded single-user block intended to require exactly one matching superuser, reset only that role, and emit safe role/verifier markers before server startup.

Protected output was classified as containing a PostgreSQL SQL error, so execution stopped before server startup, TLS/SCRAM authentication, migrator/runtime inspection, database creation, grants, migrations or tests. All generated credential/SQL/output files were deleted. PostgreSQL, Redis and web remained stopped. No role or database mutation is claimed.

Review of the submitted SQL found a likely procedure defect: it applied `min(rolname)` directly to PostgreSQL's internal `name` type while selecting the unique target. The protected server diagnostic was deleted as required, so this is a source-level hypothesis rather than claimed runtime proof. Replace that expression with a type-safe exact-single-row selection and validate the non-secret block shape before any separately authorized retry. V1 remains `BLOCKED_SINGLE_USER_SQL`; V2 remains conditionally authorized but unstarted.

#### V1h corrected actual-superuser reset — BLOCKED_ROLE_NAME_MISMATCH 2026-10-20

The user authorized a corrected two-stage attempt. A read-only single-user query using `rolname::text` proved exactly one Bidora-prefixed superuser and supplied its exact name only to protected process state. A second single-user invocation reset that exact role to a fresh 32-byte hex credential, produced no classified SQL error, and proved a login-capable SCRAM verifier through a safe marker. The unchanged cluster then started and the exact role authenticated successfully over verified TLS/SCRAM as the sole superuser.

Before database creation, the mandatory six-role guard checked the historical `bidora_local_*_migrator/runtime` names and failed. Execution stopped immediately. No disposable database or database grant was created, and no migration/test ran. PostgreSQL was stopped; the temporary superuser credential and every protected SQL/output file were removed. Redis and web remained stopped. The reset itself succeeded, so the actual superuser now has an inaccessible random SCRAM verifier after temporary-file deletion; existing migrator/runtime credentials and database state were not changed.

The failed guard is evidence that the live integration endpoint's exposed DSN users do not match the retired `bidora_local_*` fixture names (or otherwise do not satisfy that obsolete exact-name query). Do not reuse historical role names. The next preflight must derive the six role names from the existing mode-0600 integration DSNs, require six distinct safe identifiers, and inspect their actual attributes/memberships read-only before any new reset or database effect. V1 is `BLOCKED_ROLE_NAME_MISMATCH`; V2 remains unstarted.

#### V1i live-role provisioning and clean-migration start — BLOCKED_GOOSE_COMMAND 2026-10-20

The user authorized DSN-derived role inspection and conditional provisioning. The stopped-cluster read-only preflight derived six distinct safe PostgreSQL identifiers from the private DSNs and proved all six roles exist, can log in, are non-superuser, lack `CREATEDB`, `CREATEROLE`, replication and bypass-RLS attributes, and have zero role memberships. The actual sole Bidora-prefixed superuser was reset through protected single-user SQL, emitted the safe SCRAM marker and authenticated over verified TLS/SCRAM.

Exactly three disposable databases were created, owned by that bounded superuser: one clean control target and two clean tenant targets. PUBLIC database privileges were revoked. Each corresponding migrator received only database `CONNECT, CREATE`; each runtime received only `CONNECT`. No role or membership was created and no existing role attribute changed. Each migrator then created its private `bidora_migrations` ledger schema; runtime was proved to have database CONNECT but no database/public-schema CREATE. The new databases contain no business migration.

The first migration command then failed before connecting: an offline `go build` from the backend module could not resolve `github.com/pressly/goose/v3/cmd/goose` as a package provided by that module's dependency graph. No Goose migration ran. Inspection afterwards found the pinned command source and module archive already cached at `github.com/pressly/goose/v3@v3.29.0`; a later authorized retry can build from that pinned module's own `cmd/goose` directory rather than from the backend module.

Execution stopped at that failure. PostgreSQL was stopped; temporary superuser credential, derived DSN file and incomplete tool output were removed. The three disposable databases and empty private ledger schemas remain for continuation. Redis and web remained stopped. V1 is `BLOCKED_GOOSE_COMMAND`; V2 remains unstarted.

#### V1j clean-database migration — VERIFIED_CLEAN_MIGRATION 2026-10-21

The user authorized resuming with cached pinned Goose v3.29.0 source. The command built offline from its own cached module directory; the linker emitted the known macOS deployment-version warning, and the command ran from the exact cached `github.com/pressly/goose/v3@v3.29.0` source despite its self-reported development build label. Only PostgreSQL was started. Six temporary DSNs were derived from the private integration environment by replacing only the database path and adding the reviewed `bidora_migrations,pg_catalog` search path; they were mode 0600 and never printed.

Fresh migration results:

- disposable control database: migrations 00001–00013 applied; compatibility and private Goose ledger reached 13;
- disposable tenant A: common 00001–00005 and Tenderpreneur 00001–00026 applied; compatibility/ledgers reached 5/26;
- disposable tenant B: common 00001–00005 and Tenderpreneur 00001–00026 applied; compatibility/ledgers reached 5/26;
- all five exact-ceiling reruns were clean no-ops;
- control migration 14 and tenant expansion migrations after Tenderpreneur 26 were proven absent.

This is real clean-database PostgreSQL 15.19/TLS migration evidence for the frozen Option A ceilings. No Redis/web/application process ran, no post-freeze migration ran, and no V2 work began. PostgreSQL was stopped; temporary DSNs and the temporary Goose binary were removed. The three migrated disposable databases remain for the rest of V1.

V1 is not complete: deployment-specific runtime grants for all frozen operations, concurrent migration-runner behavior, rollback fault injection and restore-to-new-target evidence remain. V2 remains conditionally authorized but unstarted.

#### V1k runtime-grant comparison attempt — STOPPED_ORCHESTRATION_DEFECT 2026-10-21

The user authorized only the runtime-grant slice. PostgreSQL started, and all six original plus six clean migrator/runtime DSNs connected over TLS. Before reading or applying an ACL manifest, the orchestration attempted to place URI DSNs inside colon-delimited tuples. URI colons corrupted tuple parsing and caused a false original/clean role-pair mismatch.

Execution stopped at that first failure. No schema/table/sequence grant was read or applied, and no compatibility/DDL/TRUNCATE/ledger probe ran. PostgreSQL was stopped and temporary derived DSNs/manifests were removed. Redis and web remained stopped. The clean databases retain only their prior database-level provisioning grants and migrated schemas.

This is an orchestration defect, not a database-role mismatch. A retry must pass DSNs and role names as separate shell variables or arrays and must never serialize secret-bearing URIs through a delimiter. V1 runtime-grant verification remains incomplete; concurrent runner, rollback, restore and V2 were not started.

#### V1l corrected grant retry — STOPPED_PSQL_BINDING_DEFECT 2026-10-21

The user authorized one corrected retry. PostgreSQL started and all twelve original/clean DSNs connected. DSNs and roles remained separate variables, eliminating the prior delimiter defect. The first database-privilege precheck then used a psql variable reference inside a `-c` command; psql did not expand `:'rt'` in that invocation form, and PostgreSQL returned a syntax error at the literal colon.

Execution stopped before generating, reading or applying any ACL manifest. No runtime grant or database state changed, and no compatibility/denial probe ran. PostgreSQL was stopped and all temporary DSN/manifest files were removed. Redis and web remained stopped.

A retry must send every variable-bearing SQL statement through psql standard input or a protected file, where psql variable substitution is active, rather than through `-c`. V1 runtime grants remain incomplete; concurrent runners, rollback, restore and V2 were not started.

#### V1m runtime database-privilege baseline — BLOCKED_ORIGINAL_ACL 2026-10-21

The user authorized another corrected grant retry. PostgreSQL started, all twelve original/clean DSNs connected, DSNs/roles remained separate, and all variable-bearing SQL used psql standard input. The first substantive baseline check required the original control runtime to have database CONNECT while lacking database CREATE and TEMP.

That combined original-control assertion failed. Execution stopped before generating, reading or applying any schema/table/sequence ACL manifest. No clean runtime grant or database state changed, and no compatibility, DDL, TEMP, TRUNCATE or ledger probe ran. PostgreSQL was stopped and temporary DSN/manifest files were removed. Redis and web remained stopped.

This is no longer an orchestration defect: the original synthetic control runtime's database-level privileges do not match the required least-privilege shape. The failed combined assertion does not identify which component differs, so no specific excess/missing privilege is claimed yet. Do not copy the original ACL baseline or silently normalize it. A separately authorized read-only decomposition must report CONNECT/CREATE/TEMP booleans for each original and clean runtime before any grant change. V1 is `BLOCKED_ORIGINAL_ACL`; later V1 slices and V2 were not started.

#### V1n database-privilege decomposition — VERIFIED_DEFECT 2026-10-21

The user authorized a read-only inspection only. PostgreSQL started, six runtime DSNs connected, and separate effective/PUBLIC database privilege booleans were read for original and clean control/Tenant A/Tenant B databases. No grant or other SQL mutation ran.

Results were identical within each group:

| Targets | Runtime CONNECT | Runtime CREATE | Runtime TEMP | PUBLIC CONNECT | PUBLIC CREATE | PUBLIC TEMP |
|---|---|---|---|---|---|---|
| Original control, tenant A, tenant B | allowed | denied | **allowed** | denied | denied | **allowed** |
| Clean control, tenant A, tenant B | allowed | denied | denied | denied | denied | denied |

The original runtimes receive TEMP solely through PUBLIC. This is a concrete least-privilege defect and explains V1m. The clean targets already have the required database-level shape and must not be widened to match the original defect. Before V2 or reuse of the original synthetic databases, PUBLIC TEMP must be explicitly revoked on exactly the original three databases through their accountable owner/provisioner boundary, followed by runtime TEMP denial verification. Schema/table/sequence ACL comparison can then use the repaired original baseline.

Temporary DSNs were removed and PostgreSQL was stopped. Redis and web remained stopped. V1 remains `BLOCKED_ORIGINAL_PUBLIC_TEMP`; concurrent runners, rollback, restore and V2 were not started.

#### V1o original PUBLIC TEMP correction — VERIFIED_DATABASE_ACL 2026-10-21

The user authorized only the three-original-database correction. With the cluster stopped, the procedure derived three distinct safe original database names from the private runtime DSNs, reset/authenticated the unique actual Bidora superuser through the protected SCRAM/TLS flow, and executed exactly three statements: `REVOKE TEMPORARY ON DATABASE ... FROM PUBLIC` for original control, tenant A and tenant B.

Post-change runtime checks on every original database proved:

- runtime CONNECT allowed;
- runtime CREATE denied;
- runtime TEMP denied;
- PUBLIC CONNECT, CREATE and TEMP denied.

No clean database, schema, table, role, membership or other grant changed. PostgreSQL was stopped and all temporary credential/SQL/output/DSN files were removed. Redis and web remained stopped.

The original and clean targets now share the required database-level runtime shape. V1 runtime schema/table/sequence ACL mirroring and denial probes remain incomplete, followed by concurrent runners, rollback fault injection and restore. V2 remains unstarted.

#### V1p runtime object grants and denial probes — VERIFIED_RUNTIME_GRANTS 2026-10-21

The user authorized only the runtime object-grant slice. PostgreSQL started, all twelve original/clean migrator/runtime DSNs connected, and normalized schema/table/sequence ACL manifests were generated from each corrected original database. The corresponding clean migrator applied only those statements, after which regenerated clean manifests were byte-identical to their originals:

- control: 21 grant statements, SHA-256 `414c57c661a7d32203e24973d5b1c6448efcae0514be8d155b4f033a3363a05c`;
- tenant A: 15 grant statements, SHA-256 `b581b219a6179d7586542b437433a57e5fdc041fbe5d47a5f65fd30ca185bd4e`;
- tenant B: 15 grant statements, SHA-256 `6a45454b4ab52405fbd1ed28fd8d553a7e3b1e3f4f0d56219017cb2c1c17d055`.

Runtime positive reads returned exact compatibility control 13 and tenant common 5/Tenderpreneur 26 in both tenant databases. Twenty negative probes were then denied: CREATE SCHEMA, CREATE TEMP TABLE, ALTER TABLE, DROP TABLE, TRUNCATE and private control ledger access, plus the equivalent tenant probes including both private common/Tenderpreneur ledgers. Every mutating denial probe was transaction-contained; none unexpectedly succeeded.

PostgreSQL was stopped and temporary DSNs, manifests and redacted denial outputs were removed. Redis and web remained stopped. V1 runtime grants are verified for the frozen clean targets. Concurrent migration runners, rollback fault injection and restore-to-new-target evidence remain; V2 was not started.

#### V1q concurrent-runner setup — STOPPED_SHELL_DEFECT 2026-10-21

The user authorized only concurrent migration-runner evidence. Through the protected actual-superuser flow, three new disposable race databases were created with PUBLIC revoked and only the corresponding migrator receiving CONNECT/CREATE. Each migrator created an empty private `bidora_migrations` ledger schema. Cached Goose v3.29.0 built offline successfully.

The first pair helper then declared `label` and output paths that referenced `label` within the same shell `local` declaration under `set -u`. Bash raised an unbound-variable error before either Goose process launched. No migration, ledger row or business schema was created in the race databases.

Execution stopped. PostgreSQL was stopped and temporary superuser credential, DSNs, Goose binary and outputs were removed. The three race databases and empty ledger schemas remain. Redis and web remained stopped. A retry must split local variable declarations so `label` is assigned before constructing output paths. V1 concurrent-runner evidence remains incomplete; rollback, restore and V2 were not started.

#### V1r corrected concurrent-runner retry — BLOCKED_GOOSE_NO_LOCK 2026-10-21

The user authorized one corrected retry. The existing three targets were confirmed to have no application tables, cached Goose v3.29.0 rebuilt offline, and the pair helper assigned each local variable separately before use.

The first control pair launched concurrently. One runner exited successfully and the other exited nonzero with a protected-output classification of `duplicate_key`. Execution stopped before either tenant pair, ledger/compatibility acceptance or no-duplicate verification. The control race database is therefore partially migrated; the tenant race databases still have only their empty private ledger schemas.

Source inspection of pinned Goose v3.29.0 explains the failure: provider locking is disabled unless callers explicitly configure `WithSessionLocker` or `WithLocker`; the stock CLI used by this evidence does not configure either. Running concurrent stock CLI processes cannot satisfy the required both-successful invariant reliably. This is an application migration-runner gap, not accepted concurrent evidence.

PostgreSQL was stopped and temporary DSNs, Goose binary and protected outputs were removed. Redis and web remained stopped. V1 is blocked pending an explicit choice to implement a bounded Go migration command using Goose's PostgreSQL session locker, or to weaken the both-successful requirement (not recommended). Rollback, restore and V2 were not started.

#### V1s locked migration command and concurrent runners — VERIFIED_CONCURRENT_RUNNERS 2026-10-21

The user authorized a bounded correction and retry. `backend/cmd/migrate` is now the only application-owned migration entry point: reviewed SQL is embedded into its binary; the DSN is accepted only from `BIDORA_MIGRATION_DSN`; exact allowlisted plans are control 13, tenant-common 5 and tenant-Tenderpreneur 26; safe fixed errors do not include causes or DSNs; and API/pilot runtime packages do not import the runner or embedded migrations. Goose v3.29.0 is pinned as an application dependency solely for this command.

Before invoking Goose, the runner reads the current database OID and combines it with a fixed migration-set key to configure Goose's PostgreSQL `WithSessionLocker`. This serializes the same set in the same database without globally serializing unrelated tenant databases or migration sets. The command has a ten-minute bounded context and two-connection database pool. Runtime startup remains migration-free.

The three disposable race databases were dropped and recreated, PUBLIC privileges were revoked, corresponding migrators received only CONNECT/CREATE, and each private ledger schema was recreated. Five pairs were then launched in dependency order: control, tenant A common/Tenderpreneur, and tenant B common/Tenderpreneur. All ten command processes exited successfully and emitted only their safe set/target confirmation.

Final evidence:

- control ledger: maximum 13, 13 applied positive versions, zero duplicate versions, compatibility 13;
- each common ledger: maximum 5, five applied positive versions, zero duplicate versions, compatibility 5;
- each Tenderpreneur ledger: maximum 26, 26 applied positive versions, zero duplicate versions, compatibility 26;
- zero granted advisory locks remained after completion.

`gofmt`, offline `go mod tidy`, and offline Go 1.27.2 compilation of `./cmd/migrate` passed. This is live synthetic concurrent-runner evidence, not rollback, restore, UAT or deployment acceptance. PostgreSQL was stopped and temporary credentials, DSNs, binary and outputs were removed. Redis and web remained stopped. V1 now requires rollback fault injection and restore-to-new-target evidence; V2 was not started.

#### V1t rollback fault setup — STOPPED_INVENTORY_QUERY_DEFECT 2026-10-21

The user authorized fault injection on reset disposable targets. The locked migration command built, all three race databases were reset, PUBLIC privileges were revoked, corresponding migrator CONNECT/CREATE was restored and empty private ledger schemas were created.

Execution stopped during the first pre-fault control inventory. The normalized object query concatenated `pg_class.relkind` (PostgreSQL internal `"char"`) directly with text, so PostgreSQL rejected the expression as an ambiguous `text || "char"` operator. This occurred before installing an event trigger or invoking the migration command. Consequently there was no injected failure, migration attempt, ledger row or application schema on any target.

PostgreSQL was stopped and temporary superuser credential, DSNs, migration binary and outputs were removed. The three reset databases retain only private ledger schemas. Redis and web remained stopped. A retry must cast `c.relkind::text` in the inventory query. Rollback evidence remains incomplete; restore and V2 were not started.

#### V1u corrected rollback fault injection — VERIFIED_ROLLBACK 2026-10-21

The user authorized one corrected retry. The existing reset targets were confirmed empty, the locked migration command rebuilt offline, and normalized object inventories cast `pg_class.relkind::text` explicitly.

Five fault/retry cycles ran in dependency order: control, tenant A common/Tenderpreneur, and tenant B common/Tenderpreneur. For each cycle, a temporary superuser-owned event trigger allowed private Goose ledger-table creation but rejected the first application `CREATE TABLE`. Every injected command exited nonzero with only `migration_failed code=migration_execution_failed`; each corresponding ledger contained zero applied positive versions; and byte comparison proved the application schema/relation inventory was unchanged from its pre-run snapshot. The trigger and function were then removed before the clean retry.

All five clean retries succeeded. Final state was exact and duplicate-free:

- control ledger and compatibility: 13;
- both tenant-common ledgers and compatibility rows: 5;
- both Tenderpreneur ledgers and compatibility rows: 26;
- zero duplicate versions, temporary event triggers, temporary trigger functions or granted advisory locks.

This proves transaction rollback for the injected first application-table failure and clean recovery for every frozen migration set. It does not prove restore, arbitrary failures at every statement, UAT or deployment acceptance. PostgreSQL was stopped and temporary credentials, DSNs, binary, inventories and outputs were removed. Redis and web remained stopped. V1 now requires only restore-to-new-target evidence; V2 was not started.

#### V1v restore-to-new-target attempt — STOPPED_MANIFEST_NORMALIZATION_DEFECT 2026-10-21

The user authorized the final V1 restore slice. Three new disposable restore databases were created without replacing any source. PUBLIC privileges were revoked and only each corresponding migrator received CONNECT/CREATE. PostgreSQL 15.19 produced three mode-0600 custom-format dumps from the verified race sources, and all three restored successfully with `--single-transaction --exit-on-error --no-owner` through their reviewed migrator identities.

Execution stopped at the first source/restored control schema comparison. The comparison incorrectly treated raw `pg_dump --schema-only` text as a normalized manifest. It differed for two known representational reasons: each dump emitted a different random `\\restrict` guard, and PostgreSQL's dump/restore round trip flattened redundant nested parentheses in CHECK expressions. The sampled differences retained the same constraint names and predicates but were not byte-identical text. This is a verifier normalization defect; it is not evidence of either semantic schema equality or inequality.

Tenant manifest comparisons, exact ledger/compatibility checks and runtime denial probes were not started. The three restored databases remain isolated and unaccepted; source databases were not mutated. PostgreSQL was stopped and all temporary credentials, DSNs, dumps and manifests were removed. Redis and web remained stopped. A retry must compare catalog-derived normalized object/column/index/constraint/ACL manifests, normalize CHECK-expression presentation explicitly, and compare deterministic data manifests rather than raw dump guards. V1 restore evidence remains incomplete; V2 was not started.

#### V1w corrected restore comparison setup — STOPPED_EXPECTED_TEMP_DENIAL 2026-10-21

The user authorized a catalog-derived comparison retry against the existing restored databases. All six source/restored migrator connections succeeded. The protected verifier was prepared to compare schemas, relations, columns, indexes, PostgreSQL-reparsed CHECK constraints, other constraints, triggers, functions, ACLs, sorted row JSON and sequence state.

Execution stopped before producing the first control manifest. CHECK canonicalization attempted to create temporary clone tables through the source migrator identity, but the already-verified hardened database ACL correctly denies that identity TEMPORARY. PostgreSQL returned `permission denied to create temporary tables`. This is an evidence-runner identity defect, not a semantic mismatch and not a reason to grant TEMP to the migrator.

No persistent source or restored object changed. Tenant comparisons, ledgers, compatibility and runtime-denial probes were not started. PostgreSQL was stopped and temporary credentials, DSNs, verifier SQL and manifests were removed. Redis and web remained stopped. A retry must run read/catalog/data inspection and transaction-scoped temporary CHECK reparsing through the already protected unique superuser evidence boundary, while retaining migrator/runtime denial unchanged. V1 restore evidence remains incomplete; V2 was not started.

#### V1x protected-superuser restore comparison — STOPPED_CATALOG_CHAR_CAST 2026-10-21

The user authorized the protected-superuser retry. The unique superuser authenticated over verified TLS to all six source/restored databases, and the catalog/data queries remained transaction-scoped with no persistent DDL.

Execution stopped during the first control catalog manifest. Although `pg_class.relkind` was cast correctly, the relation manifest also concatenated the internal PostgreSQL `"char"` fields `relpersistence` and `relreplident` directly with text. PostgreSQL rejected the ambiguous operators. The transaction aborted, discarding its temporary manifest and CHECK-canonicalization objects. No semantic source/restored comparison was reached.

No persistent object or grant changed. Tenant comparisons, ledgers, compatibility and runtime-denial probes were not started. PostgreSQL was stopped and temporary credentials, DSNs, verifier SQL and manifests were removed. Redis and web remained stopped. A retry must explicitly cast all three `pg_class` internal-character fields (`relkind`, `relpersistence`, `relreplident`) to text before concatenation. V1 restore evidence remains incomplete; V2 was not started.

#### V1y final protected restore comparison — VERIFIED_RESTORE 2026-10-21

The user authorized one final retry through the protected unique-superuser evidence boundary. Every internal PostgreSQL `"char"` field used by the verifier was explicitly cast to text, including relation, column, constraint, trigger and function metadata. The superuser connected over verified TLS to all six source/restored databases; it was used only for catalog/data reads, transaction-scoped temporary CHECK reparsing and later `SET ROLE` probes.

The first data-manifest invocation showed that PostgreSQL rejects CREATE TEMP TABLE in a READ ONLY transaction. This was not a semantic mismatch and created no object. Under the already authorized temporary-object boundary, the protected data transaction changed to READ WRITE, created only a temporary manifest table and ended with ROLLBACK. A later privilege-shape query also moved from `-c` to psql standard input because variable interpolation is not active in that invocation form. Neither verifier correction persisted a database effect.

Final source/restored comparisons matched byte-for-byte after catalog normalization:

- control catalog/ACL SHA-256 `88acfacb1d81672a3e7a9a7c06d35058f56ddf99f0921e9a2584e92539855c78`; deterministic data SHA-256 `00eb373fc340aaf90b2c6362d34d0a0c3aad01b7edc626bfaf7010a0796116cf`;
- tenant A catalog/ACL SHA-256 `be61ca4e22349b633f4c88365ad8a4eb940b0f8737b38038329553a6f7d8b1dc`; deterministic data SHA-256 `d23a119acd0303eb111002084e2c5ea5e12d1a6851dcbec50898786a7733999f`;
- tenant B catalog/ACL SHA-256 `dbb9879a247c9a2029b60223cfdad98772aa7ed904573b0dbfdd9cc1334ec8fc`; deterministic data SHA-256 `a474d055d2c374dbc1f13463fa4bd25cdc1a1091b6a315dff19d33dde0997b14`.

The catalog manifests covered database/schema/relation/column/function ACLs, default ACLs, schemas, relations, columns, indexes, constraints, triggers and functions. CHECK expressions were reparsed by PostgreSQL on temporary clone tables in rolled-back transactions, accepting PostgreSQL-equivalent associative parenthesization without text heuristics. Data manifests compared sorted JSON representations of every non-system table and exact sequence state.

Restored private ledgers and compatibility were exact and duplicate-free: control 13 and both tenants common 5/Tenderpreneur 26. All three restored targets matched their migration-only race sources with runtime database CONNECT/CREATE/TEMP false. Through superuser `SET LOCAL ROLE`, all 20 restored-runtime CREATE SCHEMA, CREATE TEMP TABLE, ALTER, DROP, TRUNCATE and private-ledger probes were denied. These restore targets remain isolated evidence databases and are not approved cutover targets; current runtime authorization/grants would require separate reconciliation before any cutover.

No persistent verifier object, event trigger or advisory lock remained. PostgreSQL was stopped and temporary credentials, DSNs, verifier SQL, protected manifests and denial outputs were removed. Redis and web remained stopped. V2 was explicitly not started.

V1 is now `VERIFIED_TECHNICALLY`: existing upgrade, clean migration/no-op reruns, least-privilege database and object grants, compatibility, concurrent locked runners, injected rollback and restore-to-new-target evidence all passed at frozen 13/5/26. This does not mark BID-BE-001–007, UAT, release or acceptance complete. V2 remains only conditionally authorized as the next separate packet; V3+ remain frozen.

### V2 — Identity, browser session and MFA prerequisite

Verify registration/verification, password and optional Google login, password recovery, session rotation/revocation, CSRF/Origin, trusted-proxy resolution and live Redis limits. For BID-ADR-023 verify RFC 6238 vectors, AES-GCM AAD/key versions, enrollment, login deferral, strict JSON, TOTP replay, recovery-code single use, HMAC limiter scopes, fifth/subsequent `429`, session fixation resistance and governed-action TOTP AAL2 denial/allow behavior.

Required before authenticated BID-BE API evidence.

#### V2a dependency and synthetic-fixture preflight — BLOCKED_MFA_FIXTURES 2026-10-21

The user authorized inventory only, with no provider contact, route activation or V3 work. PostgreSQL 15.19 started and all three existing runtime identities connected over verified TLS; compatibility was exact at control 13 and tenant common 5/Tenderpreneur 26 in both tenant databases. Redis 8.6.1 started TLS-only and returned authenticated `PONG` through the constrained integration ACL. Both services were stopped immediately after the probes; web was not started.

The repository contains an in-process authenticated-STARTTLS SMTP fixture used by live pilot registration/invitation composition and adapter tests. No deployment SMTP values exist in the private integration environment, but they are not needed for provider-free technical verification. The repository also contains deterministic synthetic Google transports with locally generated RSA/JWKS responses and live PostgreSQL/Redis HTTP-flow composition; no Google deployment credentials exist or are required for provider-free technical verification. Neither external provider was contacted.

The first missing prerequisite is the MFA verification fixture/suite required by V2. Centralized `backend/tests/` contains zero MFA-named test files and zero assertions referencing RFC 6238/TOTP/recovery-code/AAL2 behavior. Consequently there is no authorized deterministic fixture currently able to prove key-version/AAD handling, enrollment, login deferral, replay denial, recovery-code single use, limiter thresholds, session rotation or governed-action AAL2 denial/allow as one V2 evidence packet. Existing session/CSRF and identity tests do not substitute for those missing MFA assertions.

The private integration environment also contains no persisted MFA keys, SMTP values, Google values or identity fixture secrets; this is correct for synthetic test-owned material and no secret was invented. A preflight deviation was identified: the Redis probe supplied the password to `redis-cli -a` through shell expansion. It was not printed or persisted, but could briefly be visible in the local process argument list. Any later run must use `REDISCLI_AUTH` or an equivalent non-argv secret channel.

V2 stopped at this first missing prerequisite. No test suite ran, no source/schema/route changed and no V3 work started. Implementing the centralized deterministic MFA suite is a separately authorized code-generating slice.

#### V2b centralized MFA verification suite — VERIFIED_MFA_PREREQUISITE 2026-10-21

The user authorized centralized deterministic MFA tests and permitted only bounded production correction if those tests exposed a defect. Eleven tests were added across `backend/tests/identity`, `backend/tests/httpapi`, `backend/tests/opportunity` and `backend/tests/integration`; all use external test packages and synthetic secrets.

Verified behavior:

- six RFC 6238 SHA-1 timestamps using the approved six-digit truncation, malformed-code rejection and accepted-counter replay denial;
- AES-256-GCM current/historical key-version decryption plus user/factor AAD and unknown-version rejection;
- ten unique bounded recovery codes with case-insensitive domain-separated hashes;
- enrollment stores only encrypted pending material while returned base32/URI material matches the AAD-bound secret;
- password login with active MFA returns only a short-lived MFA challenge and creates no pre-MFA session;
- successful TOTP and recovery-code step-up return replacement session/CSRF material, bind the replacement to the current session commit, reject TOTP counter replay and consume a recovery code once;
- login challenge completion commits the exact returned session hashes and rejects challenge replay;
- deterministic application limiting denies the fifth failure and subsequent attempts;
- the live TLS Redis MFA limiter independently proves the fifth failure establishes the ten-minute denial and later Begin calls fail closed;
- strict MFA HTTP JSON rejects unknown, duplicate and trailing fields before invoking the use case;
- proposal approval denies PRIMARY_ONLY, RECOVERY_CODE AAL2 and stale TOTP, while allowing recent TOTP AAL2.

The strict HTTP test exposed a bounded production defect: shared `bindWorkflow` writes a 4xx response through Fiber but returns nil after successful JSON serialization, so MFA handlers continued into the use case and overwrote rejection responses. `internal/httpapi/mfa.go` now checks the response status after binding and returns immediately for an already-written 4xx/5xx response in all six MFA handlers. The focused test then passed. Unrelated handlers that use the shared helper were not changed; their strict-binding behavior remains a later API-verification risk.

Actual checks:

- compile-only discovery for identity, HTTP API, opportunity and integration test packages passed;
- focused deterministic MFA/assurance tests passed in all three non-live packages;
- the single new live Redis MFA threshold test passed against Redis 8.6.1 TLS and Redis was stopped immediately afterward.

PostgreSQL and web remained stopped; no external provider was contacted, no schema/route activation occurred and V3 was not started. This verifies the MFA prerequisite suite, not all V2 identity/session behavior. V2 remains active and incomplete until its remaining registration, verification, password/Google login, recovery, session/CSRF/proxy and live integration evidence is selected and executed.

#### V2c password registration and email verification — BLOCKED_PILOT_FIXTURE_COMPOSITION 2026-10-21

The user authorized focused registration/verification evidence against the synthetic PostgreSQL, Redis and in-process authenticated-STARTTLS SMTP dependencies. Centralized identity tests for registration, self-registration persistence, email-verification issuance/consumption, SMTP STARTTLS and token handling passed. Centralized HTTP tests for strict registration, email-verification and login boundaries passed.

The focused live integration package then failed in both pilot registration composition tests at `pilot.Open` with the fixed safe error `pilot_deployment_open_failed`. Verification stopped there. Source inspection identified the first exact fixture defect: shared `recoveryDeploymentConfig` does not provide the now-mandatory synthetic `MFAKeyRing`, so `pilot.Build` returns `pilot_runtime_invalid` before probing PostgreSQL, Redis or SMTP. The same stale shared fixture also does not configure MinIO, OpenAI or ClamAV; full pilot readiness now requires those unrelated document-processing dependencies and object-storage probing. Adding or starting those services is outside this bounded identity slice and is not inferred from test authorization.

The integration command is failed as a group; its other selected tests are not claimed as passing from that run. `recoveryDeploymentConfig` inserts random synthetic tenant/cluster/assignment compatibility rows before `pilot.Open` and has no cleanup, so the two failed composition cases may have left synthetic registry rows in the control fixture; verification did not restart PostgreSQL to inspect or remove them after the stop condition. No external provider was contacted. The cleanup trap stopped PostgreSQL and Redis, web remained stopped, and neither Google verification nor V3 began. V2 remains blocked on selecting a bounded composition correction rather than fabricating unrelated provider infrastructure.

#### V2d shared identity composition and rerun — BLOCKED_VERIFIED_LOGIN_503 2026-10-21

The user authorized a bounded composition correction without weakening full pilot readiness. `pilot.composeIdentity` now owns the existing password/MFA-backed login, registration, verification and recovery assembly. Full `pilot.Build` calls that same composer and still requires tenants, MinIO, OpenAI, ClamAV and all existing probes. An unmounted `BuildIdentityRuntime` plus `httpapi.NewIdentityVerificationApp` exposes only login, registration and email verification for centralized evidence; it neither starts services nor mounts production routes.

A guarded cleanup transaction selected the two newest fixture sets created within two hours using all fixed synthetic recovery tenant/secret/endpoint markers, required zero membership and invitation references, deleted exactly their compatibility rows, assignment, unique cluster and tenant, and reported `orphan_fixture_sets_removed=2`. No identifiers or credentials were printed.

The rerun passed the focused centralized identity and HTTP suites. In the live shared-composition test, generic registration, authenticated-STARTTLS delivery, exact token-hash persistence, unverified-login denial, atomic verification, replay denial and generic duplicate registration all reached their assertions. The subsequent verified-password login returned safe HTTP 503 instead of 204, so verification stopped at that first failure. The live integration package remains failed as a group and the other selected live tests are not claimed from that run. The response body/cause was not exposed or probed after the stop condition.

PostgreSQL and Redis were stopped by the cleanup trap; web remained stopped. No external provider, MinIO, OpenAI, ClamAV, Google flow or V3 work ran. V2 remains blocked on bounded diagnosis of the safe verified-login failure.

#### V2e verified-login 503 diagnosis — VERIFIED_BOUNDED_FIX 2026-10-21

The single failing shared-composition test was instrumented to disclose only the stable public error code. It reproduced HTTP 503 `AUTHENTICATION_UNAVAILABLE`; PostgreSQL/Redis were stopped immediately. A subsequent minimized migrator inspection of only the newest synthetic identity showed: verified identity, one consumed verification challenge, zero sessions, zero login audits and zero active MFA factors. The control runtime could insert sessions and audit rows but lacked `SELECT` on `identity.mfa_factors`.

This was the exact fail-closed cause: every current password login invokes `MFAService.BeginLogin`, whose `HasActiveMFA` query must read that table even when the user has no factor. Through the existing control migrator boundary, the integration control runtime received only `GRANT SELECT ON identity.mfa_factors`; the grant was verified without printing the role or credentials. No schema, role attribute, membership or broader table privilege changed.

The authorized single test then passed. It now additionally verifies one committed active session joined to one `UserLoggedIn` audit row and inspects only its random Redis prefix to prove successful login retained no identity failure/block, network block or reservation key. The test already covers generic registration, in-process authenticated STARTTLS delivery, exact token-hash persistence, unverified-login denial, atomic single-use verification, replay denial, duplicate-generic behavior and verified login cookies.

This exact original-control ACL now differs from the historical V1 original/clean byte-identical manifest; the V1 evidence remains true for its recorded time, but current deployment-grant inventory must include this least-privilege addition before future clean/restore targets are treated as equivalent. The rest of the previously selected live registration tests were not rerun in this diagnosis-only command, so the broader V2 registration/verification slice is active rather than complete.

PostgreSQL and Redis were stopped after each operation; temporary diagnostic/grant source files were removed. Web stayed stopped. No external provider, Google flow or V3 work ran.

#### V2f remaining registration live evidence — VERIFIED_TECHNICALLY 2026-10-21

The user authorized exactly three remaining centralized live tests. Against the synthetic PostgreSQL 15.19 TLS control target and Redis 8.6.1 TLS endpoint, all passed in one focused run:

- `TestLiveEmailVerificationIssuanceResendAndAtomicConsumption`;
- `TestLiveRegistrationRedisLimitsAreAtomicAndDoNotClear`;
- `TestLiveSelfRegistrationCreatesUnverifiedIdentityWithoutMembership`.

Combined with the earlier passing centralized identity/HTTP suites and the repaired shared-composition test, the bounded password-registration/email-verification slice now has technical evidence for strict binding, approved identity/network limits, generic new/existing `202`, no membership grant, hash-only challenge persistence, resend invalidation, authenticated-STARTTLS delivery, verified-only password login, atomic single-use verification and generic replay denial.

PostgreSQL and Redis were stopped by the cleanup path and independently reported stopped. No external provider, SMTP provider, Google flow or V3 work ran. This completes only the V2 password-registration/email-verification slice; password recovery/change, broader login/session/CSRF/proxy behavior, Google and other V2 criteria remain separately unverified.

#### V2g password login and browser session — VERIFIED_TECHNICALLY 2026-10-21

The bounded deterministic identity suite initially stopped at `TestSessionValidatorHashesRefreshesAndReturnsCurrentPrincipal`: its synthetic “valid” record predated mandatory MFA assurance fields and therefore correctly failed closed as `session_unavailable`. The fixture was corrected to represent `AAL1`/`PRIMARY_ONLY` with a nonzero assurance instant, and its invalid-record table now also checks missing assurance level and timestamp. No production behavior changed for this correction.

The rerun passed the selected centralized identity tests for password authentication, dummy work/concealment, dependency and limiter failures, login commit/rehash/failure ordering, session policy/token hashing, idle and absolute expiry rejection, browser Origin/CSRF policy, cookie attributes, session validation, logout atomicity and rollback. Selected HTTP tests for strict login, secure cookies, browser-session middleware, CSRF-only bootstrap and logout clearing/failure behavior also passed.

`TestLiveLoginSessionTouchAndLogout` then passed against PostgreSQL 15.19 TLS and Redis 8.6.1 TLS. It proved generic wrong-password behavior, successful identity-counter reset, Argon2id rehash/version advancement, exact 30-minute idle and 12-hour absolute expiry, atomic session/audit persistence, token/CSRF-bound principal validation, idle touch, atomic logout audit/revocation and rejection of the revoked token.

PostgreSQL and Redis were stopped by the cleanup path. Web remained stopped. No Google test, tenant-capture test, external provider call or V3 work ran. This verifies only the bounded password-login/browser-session slice; password change/recovery, Google and other remaining V2 criteria stay separate.

#### V2h password change and recovery preflight — BLOCKED_MISSING_FIXTURES 2026-10-21

The user authorized the bounded password-change/password-recovery evidence slice using PostgreSQL, Redis and only the in-process authenticated-STARTTLS SMTP fixture. Centralized test inventory stopped at the first prerequisite gap: `backend/tests/` contains no password-change or password-recovery behavioral, HTTP or live integration test. The only password-change-specific test found is a database migration-text assertion; existing Google-link reauthentication, registration, login and MFA recovery-code tests do not substitute for the required operation evidence.

Missing deterministic/live evidence includes strict request binding, generic recovery request responses, Redis recovery limits, hash-only recovery challenge persistence, authenticated-STARTTLS delivery, expiry and atomic single-use reset, current-password proof, credential-version/auth-epoch advancement, all-session revocation, replacement-session commit/cookie behavior and rollback/failure ordering.

No test body ran, PostgreSQL and Redis were not started, web remained stopped, and no SMTP listener, external provider, Google flow or V3 work ran. The slice is `BLOCKED_MISSING_FIXTURES`; implementing a centralized bounded suite requires explicit code-generation authorization.

#### V2i password change and recovery suite — VERIFIED_TECHNICALLY 2026-10-22

The user authorized implementation of the missing centralized suite. Eight focused tests were added across identity application, HTTP and live integration packages. Deterministic evidence covers current-password proof before hashing/mutation, exact credential/auth-epoch increments, replacement-session commit material, generic known/unknown recovery request behavior, hash-before-delivery ordering, ambiguous delivery concealment, reset token hashing/replay concealment, limiter-before-request ordering, strict JSON and replacement cookies.

The HTTP suite exposed two bounded recovery binding defects: Fiber rejection serialization returned nil and allowed handlers to overwrite 4xx responses, and duplicate JSON keys were not rejected. `internal/httpapi/password_recovery.go` now returns immediately after an already-written rejection and applies the existing duplicate-object/UTF-8 validator. Focused HTTP tests then passed.

The first live run proved password change and its replacement session, then failed closed at recovery issuance. Minimized inspection showed the runtime lacked all privileges on `identity.password_recovery_challenges`; the migrator granted only `SELECT, INSERT, UPDATE` on that table. A second run still failed with PostgreSQL SQLSTATE 42804. Safe server classification showed `expires_at` received an inferred interval because the shared `$4` parameter was uncast in timestamp arithmetic. `queries/identity/recovery.sql` now casts the source parameter explicitly to `timestamptz`, and sqlc v1.31.1 regenerated `internal/identity/persistence/sqlc/recovery.sql.go` from source.

The final live PostgreSQL/Redis/STARTTLS test passed. It verifies current-password authentication, atomic password credential/version and auth-epoch advancement, all-session revocation, one replacement session, hash-only 30-minute recovery challenge persistence, local authenticated-STARTTLS delivery, atomic reset credential/epoch advancement, revocation of the replacement session, single-use replay denial and fourth identity recovery attempt denial after three allowed attempts.

Focused deterministic reruns, affected compile-only discovery and generated-query inspection passed. PostgreSQL and Redis were stopped, web remained stopped and temporary diagnostic/generator files were removed. No external provider, Google flow or V3 work ran. The current original-control ACL now contains the recovery-table grant in addition to the earlier MFA-factor read; historical V1 manifests remain point-in-time evidence and future clean/restore equivalence must include both reviewed least-privilege additions.

#### V2j synthetic Google sign-in — VERIFIED_TECHNICALLY 2026-10-22

The user authorized provider-free Google sign-in evidence and explicitly excluded account linking. Selected deterministic identity tests passed for state/nonce/S256 PKCE generation, transaction hashing/single consumption, exact authorization and token-exchange requests, browser-binding rejection before exchange, fixed Google transport allowlists, local RSA/JWKS signature validation and bounded refresh, strict verified-email/audience/issuer/nonce claims, subject-based identity resolution, safe authentication failures and session issuance. Link-named tests and linking HTTP/integration flows were not selected.

A new deterministic Google-login assertion proves that an enabled MFA gate returning a required challenge yields only MFA challenge material: no session/CSRF token generation and no session commit occurs before successful second factor. Existing non-MFA Google login/session tests remained passing.

Selected Google start/callback HTTP tests passed for exact Origin and empty-request constraints, state-binding cookie behavior, generic callback failure redirects, correlation-cookie clearing and secure session cookies. The two authorized live tests then passed against PostgreSQL 15.19 TLS and Redis 8.6.1 TLS using only in-process deterministic Google transport plus locally generated RSA/JWKS material: the complete sign-in callback committed a Google session and rejected replay, and the Redis OIDC transaction was single-use.

PostgreSQL and Redis were stopped by the cleanup path; web remained stopped. No external provider was contacted, no account-link operation/test ran and V3 was not started. This verifies synthetic Google sign-in only; account linking and other remaining V2 criteria stay separate.

#### V2k synthetic Google account linking — VERIFIED_TECHNICALLY 2026-10-22

The user authorized local-fixture account-linking evidence. Selected deterministic identity and HTTP tests passed for fresh-password reauthentication, same-current-user/session/auth-epoch binding, purpose-bound OIDC transactions, browser/principal swap rejection before exchange, subject uniqueness/conflict rollback, atomic external-identity binding, auth-epoch advancement, old-session revocation, replacement-session creation, generic failure redirects and replay denial. The explicit identity-resolver test also passed, proving matching email alone cannot silently link a Google subject and disabled-subject state fails closed.

Both selected live tests initially stopped during fixture setup because their raw synthetic session inserts predated mandatory MFA assurance columns. The production schema correctly rejected null assurance. Only those test inserts were corrected to explicit `AAL1`, `PRIMARY_ONLY` and a nonzero assured timestamp.

The same two live tests then passed against PostgreSQL 15.19 TLS and Redis 8.6.1 TLS using deterministic local Google transport and local RSA/JWKS. They verify fresh-password-bound HTTP initiation, state/nonce/PKCE transaction consumption, exact current principal binding, atomic subject link, credential/auth-epoch and session rotation, required audit evidence, no tenant membership grant, stale-proof rejection and callback replay rejection.

PostgreSQL and Redis were stopped by the cleanup path; web remained stopped. No external provider was contacted and V3 was not started. This verifies the bounded account-linking slice; remaining V2 criteria must still be inventoried before V2 can be declared complete.

#### V2l live MFA composition — VERIFIED_TECHNICALLY 2026-10-22

The user authorized live MFA composition while excluding governed-action expansion and V3. Least-privilege preflight found `identity.mfa_factors` had read-only runtime access while `identity.mfa_recovery_codes` and `identity.mfa_transactions` lacked runtime access. Through the existing migrator boundary, only missing `SELECT`, `INSERT` and `UPDATE` privileges were granted on those three MFA tables; no DELETE, DDL, role attribute or membership was added.

The first live enrollment stopped safely because PostgreSQL inferred the reused pending-factor timestamp parameter inconsistently between a timestamp column and interval arithmetic. The same pattern existed in MFA login-transaction expiry. `queries/identity/mfa.sql` now casts both shared source parameters explicitly to `timestamptz`; sqlc v1.31.1 regenerated `internal/identity/persistence/sqlc/mfa.sql.go` from source.

The final live PostgreSQL/Redis composition test passed. It uses the production AES-GCM key ring, production HMAC scope derivation, production Redis MFA controller, pgx/sqlc MFA store and secure session generator. It verifies encrypted pending enrollment, TOTP confirmation, ten recovery codes, auth-epoch advancement and initial session rotation; MFA-gated password login; TOTP step-up; recovery-code step-up and replay denial; recovery-code regeneration; factor replacement with historical-factor preservation, auth-epoch advancement and all-session replacement; concurrent recovery-code and TOTP attempts with exactly one winner; and concurrent MFA-login transaction completion with exactly one winner.

The production HMAC test now proves MFA purpose and target domain separation. The MFA HTTP suite now proves `mfa_attempt_denied` maps to generic 429 with `Retry-After: 600`. The dedicated live TLS Redis test was rerun and again proved the fifth failed challenge establishes denial and subsequent Begin calls remain denied.

Focused MFA identity/HTTP reruns, affected compilation and generated-query inspection passed. PostgreSQL and Redis were stopped and temporary grant/generator files removed; web remained stopped. No governed-action test was added or run and V3 was not started. Historical V1 ACL manifests are point-in-time evidence and now also predate the reviewed MFA mutation grants.

#### V2m MFA readiness and recovery preparation — VERIFIED_TECHNICALLY 2026-10-22

Centralized application tests verify that MFA readiness reads the current active-factor state, enables enrollment only when no factor is active, exposes step-up and recovery-code availability only for an active factor, derives session age, and fails closed when factor state is unavailable. Recovery preparation tests verify current credential/account facts, deterministic blockers, deployment availability and configured token/delivery/session-effect policy without unauthenticated lookup material.

Composed HTTP tests verify exact Origin and session-bound CSRF-cookie enforcement before either preparation use case, private/no-store responses, approved current-state envelopes, and redaction of user/session/factor identifiers, email values, secrets, recovery codes, tokens and provider/database diagnostics. Targeted tests and the complete centralized identity and HTTP API packages passed. A stale pilot-route test fixture was updated with inert instances for all currently mandatory handlers; no production behavior changed. No service was started, governed-action coverage was not expanded and V3 remained frozen.

#### V2n session inventory and administration — VERIFIED_TECHNICALLY 2026-10-22

Centralized application tests verify account-scoped inventory reads, server-derived current-session marking, fail-closed behavior when the authenticated session is absent, bounded revoke-other ownership/evidence inputs, current-session rejection in favor of logout, fresh-authentication enforcement, and revoke-all auth-epoch/result handling. Centralized composed HTTP tests verify exact Origin and session-bound CSRF enforcement for inventory and unsafe administration requests, current-session conflict mapping, idempotency input, revoke-other without cookie clearing, revoke-all with both secure cookies cleared, private/no-store responses, and omission of user, token, CSRF, email, network and device metadata.

Targeted tests and the complete centralized identity and HTTP API packages passed. No production file changed and no service was started. Governed-action expansion remained excluded and V3 remained frozen. This is application/HTTP acceptance evidence; live PostgreSQL race and persistence evidence remains a separate V2 gate.

#### V2o identity-only live database gates — VERIFIED_TECHNICALLY 2026-10-22

A least-privilege preflight checked each required operation independently across session commands, sessions, users, password credentials, recovery challenges, audit and outbox tables. One grant statement supplied the missing session-administration command-table runtime operations; no DELETE, DDL, role attribute or membership was added.

The final PostgreSQL 15.19/TLS tests passed. Session evidence proves account-owned inventory excludes another user, concurrent same-key revoke-other has exactly one effect and one replay, and command/audit/outbox rows commit once. Revoke-all advances auth epoch exactly once, revokes every current-epoch session while preserving the other account, records one audit/outbox pair and makes the old token invalid. Password-reset evidence races two callers against one stored token and proves exactly one winner, one generic denial, one challenge consumption, one credential/auth-epoch advancement, all-session revocation and one recovery audit row.

The first two password-reset runs stopped on defects in the synthetic direct-insert fixture (an uncast repeated timestamp and the wrong purpose literal); production reset SQL was not implicated. The fixture now uses explicit `timestamptz` casts and the schema-defined `PASSWORD_RECOVERY` purpose. PostgreSQL was stopped after every run; Redis and web were not started. No governed-action or V3 work ran.

V2 is not blocked solely by governed-action assurance. The remaining known gates are: (1) recent-TOTP-only AAL2 denial/allow evidence for submission confirmation and membership offboarding, which is the excluded governed-action work; and (2) deployment-specific trusted-proxy CIDR evidence, because only deterministic synthetic CIDRs have been verified. A final read-only V2 reconciliation follows only after those are resolved or explicitly deferred by an accepted exception.

#### V2p governed-action TOTP assurance and final reconciliation — VERIFIED_TECHNICALLY 2026-10-22

The user explicitly deferred deployment-specific trusted-proxy CIDR evidence to deployment acceptance and authorized centralized governed-action policy tests. Submission confirmation now has deterministic denial evidence for AAL1 primary-only, AAL2 recovery-code, stale TOTP and future-dated TOTP assurance, plus allow evidence only for recent TOTP AAL2. Membership offboarding begin, retarget and cancellation each have the same denial/allow matrix; denied attempts do not cross their control persistence boundaries. The complete opportunity and authorization test packages passed. No production file changed and no service started.

Read-only reconciliation against the V2 definition at the start of this section finds technical evidence for registration/verification, password and synthetic Google authentication/linking, password change/recovery, browser session rotation/revocation, CSRF/Origin, deterministic trusted-proxy resolution, Redis limits, MFA cryptography/enrollment/login/step-up/recovery/replacement/replay, account session administration, password-reset replay and all three governed operations' recent-TOTP policy (proposal approval, submission confirmation and membership offboarding). Therefore V2 is `VERIFIED_TECHNICALLY`.

This does not pass deployment acceptance, UAT, privacy/legal approval, production KMS/key custody or release readiness. Deployment must still verify its actual trusted-proxy CIDRs before route activation. V3 remains unstarted and requires a separate command.

### VER-02R — control 28 / common 9 / Tenderpreneur 32 migration resilience — VERIFIED_TECHNICALLY 2026-10-22

The user authorized clean migration, concurrent runner, rollback and restore verification at the expanded source ceilings. Only the existing Bidora-owned PostgreSQL 15.19 loopback TLS/SCRAM cluster ran. The three original application databases were not reset or used as destructive targets.

The existing disposable `clean` control and two tenant databases were transactionally reset by their dedicated migrators: only non-system application and private-ledger schemas were dropped, then the private ledger schema was recreated. No database, role, ownership or database-level grant changed. The locked standalone migrator applied fresh control 1–28 and common 1–9/Tenderpreneur 1–32 in both tenant targets. Version-1 query-derived runtime grants then applied successfully to all three clean targets, proving runtime TLS/same-database binding, exact compatibility, role shape and effective 68-control/61-tenant table privilege manifests.

Rollback fault injection used only migrator-owned disposable race targets and did not require superuser/event-trigger authority. On the control target at 13, a deliberate collision late in migration 14 proved the preceding account-type column/function/trigger work, compatibility and ledger rolled back. Tenant A common first advanced to 6; a deliberate collision on the second migration-7 table proved the first table and ledger/compatibility update rolled back. Tenderpreneur first advanced to 27; a deliberate migration-28 function collision proved both preceding proposal-table alterations rolled back. Each command returned only `migration_failed code=migration_execution_failed`; exact pre-fault compatibility/ledger and absence of preceding effects were checked before removing the synthetic collision.

The first concurrent helper attempt stopped before launching a process because a shell-local declaration referenced an unset variable. After declaration ordering was corrected, five pairs ran in dependency order: control, both common sets and both Tenderpreneur sets. All ten locked migrator processes exited successfully. Final clean/race/restore evidence is duplicate-free and exact across nine databases: every control ledger has 28 positive versions and compatibility 28; every tenant has 9 common and 32 Tenderpreneur positive versions with compatibility 9/32. Zero granted advisory locks remained.

Three existing restore targets were reset without replacing a source. PostgreSQL 15.19 `pg_dump` custom archives of the clean targets restored through corresponding migrator identities with `--single-transaction --exit-on-error --no-owner`. Environment-only libpq credentials were held in mode-0600 temporary files and never printed or placed in command arguments. Every restored set accepted an exact-target standalone-migrator no-op rerun.

Catalog-derived source/restore comparison matched byte-for-byte for schemas, relations, columns, indexes, exact PostgreSQL constraint definitions, triggers, function definitions/ACLs, default ACLs and normalized effective table ACLs, followed by deterministic sorted JSON data comparison for every non-system table. Selected matched hashes were:

- control table ACL `4f3fa8d0b00b38b631543f00c301913012ba33a052c40ddea5840f62e9025d1f`, constraints `89ce3dfc61f9bbc6a8f26561aa2d866fbf7d79cce7f4dc85600bc01fdd6ac6c7`, data `18a344b18f1f13802e9b2f38161a191640eaa6a1347e17b8c99e50051afcc6e0`;
- tenant A table ACL `8a7a0ee67a0b98ec068f425998a3dbcdede8fb05030025026127f9943ced1b02`, constraints `b770ccf758b1d332399dbbb4b72cc451dda3d2e29636b204b3ac8a9fe704b338`, data `70ae7c5ce33f7b04834a3f2e84e3de6bff5ac92cf5cd0505fdf967755f892f80`;
- tenant B table ACL `18498704ce3e40c2ded0220c4dae7d1b817d572f3a84e4bd02175aa13d0cc31b`, the same schema/constraint hash as tenant A, and data `dff8b2587c7cbb6e41e75846488f86a14289e7745d202a9040d5b11f8e39af14`.

One runtime-grant attempt on the race control target returned the fixed `runtime_unavailable` code because race/restore evidence databases intentionally lack runtime database CONNECT. It made no change and was not bypassed. Clean targets carry verified runtime database access and exact grants; restored object ACLs match their sources, but race/restore targets remain non-cutover evidence targets requiring separately reconciled database-level runtime authorization before use.

PostgreSQL was stopped and temporary DSNs, environment files, dumps, helper sources, binaries and outputs were removed. Redis, web and providers remained stopped. This proves clean migration, bounded transactional fault rollback, concurrent application-owned locking, runtime grants on clean targets and restore equivalence at 28/9/32. It is not application transaction verification, deployment acceptance, UAT or release readiness.

### V3 — BID-BE-001 transaction and safe-error foundation — VERIFIED_TECHNICALLY 2026-10-22

PostgreSQL 15.19 TLS evidence for the representative tenant capture transaction proves state/source/audit/outbox/idempotency atomicity, matching replay, conflicting-key/hash rejection, same-key concurrent convergence, stale assignment denial and zero visibility from the second tenant. Deliberate duplicate audit and duplicate outbox IDs independently failed the transaction; each left zero new state, idempotency, audit or outbox effect. Representative control evidence proves one winner under concurrent invitation consumption, deny-first invitation issue behavior and same-key concurrent session-revocation convergence without duplicate command/audit/outbox effects.

Centralized HTTP tests prove expected-version workflow conflict maps to `409 WORKFLOW_CONFLICT`, unknown SQL-like diagnostics map to redacted `503 WORKFLOW_UNAVAILABLE`, and capture idempotency/in-progress conflicts remain safe 409 responses. Opportunity, HTTP, PostgreSQL and architecture suites passed under `-race`; PostgreSQL tests include caller-cancellation and bounded-shutdown propagation. `go vet` passed for the affected production/test packages.

Only PostgreSQL ran and was stopped afterward. Redis, web and providers remained stopped. BID-BE-001 is `VERIFIED_TECHNICALLY`; independent security review, UAT and acceptance remain pending.

### V4 — BID-BE-002 opportunity and workspace journey — VERIFIED_TECHNICALLY 2026-10-23

PostgreSQL 15.19 verified the production capture transaction followed by the production control authorizer, paid-write authorization wrapper, application services and tenant workflow/read stores. Evidence covers source-preserving manual capture; concurrent same-key human verification convergence and replay; stale verification conflict; disabled-owner denial; one active workspace and replay; conflicting second workspace denial; immutable BID then NO_BID rationale/history; conflicting idempotency reuse; stale decision rejection; bounded opportunity/Bid reads and owner/state filters; 20-item duplicate hints with truncation; audit/outbox lineage; active-DENY precedence at the current context generation; stale context rejection; paid-write fail-closed behavior; changed assignment-generation rejection; direct lease mismatch; and tenant-B non-observation of exact opportunity and Bid IDs.

The first live verification exposed a production defect: all three workflow outbox queries predated tenant-common migration 5 and omitted its mandatory `actor_id`. PostgreSQL rejected the transaction under `outbox_events_actor_required`, leaving no successful workflow effect. The bounded correction added the authenticated actor to verification, workspace-created and decision outbox inserts, regenerated sqlc v1.31.1 output and advanced the reviewed query checksum. The same live test then passed under `-race`.

Centralized HTTP evidence now proves successful strict binding and 201/private-no-store responses for verification, workspace creation and decision recording in addition to the existing safe 409/503 mapping. Opportunity, HTTP, database and architecture packages passed uncached under `-race`; the selected live capture and V4 journey tests passed together under `-race`; affected `go vet`, build and canonical OpenAPI generation checks passed. PostgreSQL was the only service started and was stopped afterward. Redis, web and external providers remained stopped/uncontacted.

BID-BE-002 is `VERIFIED_TECHNICALLY`, not accepted. Independent security review, UAT, deployment acceptance and V5–V10 remain pending.

### V5 — Document pipeline and BID-BE-003 compliance — VERIFIED_TECHNICALLY 2026-10-23

PostgreSQL 15.19/TLS evidence now traverses the production control authorizer, paid-write wrapper, application services, tenant stores and processing worker. It proves PDF/DOCX allowlisting and the 25 MiB ceiling; checksum, size and immutable object-version matching; initial quarantine; exact-version reads; CLEAN, REJECTED and fail-closed/retry scan outcomes; rejected-tender analysis cancellation; manual and human-accepted cited requirements; active owner assignment; same-bid CLEAN evidence enforcement; cross-bid and tenant-B denial; unsupported repeat satisfaction denial; exclusive expiry-boundary blocking; deterministic readiness; immutable correction history; one-winner concurrent correction; stale correction conflict; and material-correction invalidation of an older technical response. A synthetic Unix-socket clamd protocol test separately exercises the production ClamAV adapter's exact INSTREAM bytes and CLEAN/FOUND/error/size-mismatch mappings. No OpenAI call was made.

Live execution exposed three production defects. Ten V5 outbox inserts predated tenant-common migration 5 and supplied a null `actor_id`; the bounded correction passes the authenticated/requesting actor for upload, scan, extraction, requirement, evidence, AI-review, correction and technical-response events, regenerates sqlc v1.31.1 output and advances reviewed query checksums. The shared read guard also rejected source-defined bounded 500/1000-row queries despite those SQL limits; its ceiling now matches the maximum reviewed 1000-row boundary. Successful HTTP-contract tests additionally exposed missing private cache controls on requirement create/assign/list/readiness responses; all now return `Cache-Control: private, no-store`. Those tests prove authenticated authoritative binding and private responses for document initiation, requirement creation/readiness and evidence satisfaction. PostgreSQL then passed the V3–V5 selected journeys together under `-race`.

Uncached race runs passed for opportunity, HTTP, database and architecture suites. Affected vet, build, formatting, checksum JSON and canonical OpenAPI generation checks passed. Only the isolated PostgreSQL fixture ran and it was stopped after verification; Redis, web, OpenAI and external MinIO/ClamAV services remained stopped/uncontacted. The in-memory object service proves application binding and the synthetic clamd server proves adapter protocol handling, not deployment-provider acceptance. Real MinIO bucket/versioning/private-policy acceptance, real ClamAV deployment acceptance, independent security review, UAT and formal acceptance remain deferred.

BID-BE-003 is `VERIFIED_TECHNICALLY`, not accepted. V6 verification follows below.

### V6 — BID-BE-004 responses, proposal and advisory AI — VERIFIED_TECHNICALLY 2026-10-23

PostgreSQL 15.19/TLS evidence traverses the production control authorizer, paid-write boundary, application services, tenant stores, entitlement checkpoints and advisory workers. It proves exact-evidence technical-response revision and bounded history; one winner under concurrent revision with a stale-write conflict; cited `PROPOSED_UNVERIFIED` response/proposal suggestions over exact CLEAN versions; stale response-draft cancellation before provider invocation; retained ambiguous-provider failure evidence; immutable proposal response/source snapshots and history; human-edited application without mutation of the original suggestion; stale application conflict; deterministic approval-readiness reporting; byte-identical PDF/DOCX rerenders with matching SHA-256; tenant-B non-observation; and authenticated actor provenance for exercised outbox facts. No financial schedule, amount, currency or pricing DTO was introduced or passed through the tested Option A HTTP/application/model contracts.

Live execution exposed three bounded production defects. Proposal and advisory-AI outbox inserts omitted mandatory `actor_id`; the authenticated/requesting actor is now passed and sqlc v1.31.1 output/checksums were regenerated. Legacy unnamed run-shape checks made migration 31/32 entitlement states unreachable, so additive migration 33 replaces only those checks while preserving terminal evidence requirements. The original global `(kind, document_version_id)` job uniqueness also made a second immutable advisory run impossible; additive migration 34 retains singleton malware-scan/tender-analysis effects while permitting repeated response/proposal advisory jobs. Compatibility, routing and runtime-grant floors now require Tenderpreneur schema 34. Both tenant fixtures upgraded through reviewed migrator credentials, and Goose v3.29.0 validation passed.

Centralized authenticated HTTP evidence proves strict binding and private/no-store responses for technical response save, proposal save, advisory response request, human suggestion application and exact-version export, including export integrity headers. The selected V3–V6 PostgreSQL journeys passed together under `-race`; opportunity, HTTP, database, architecture and runtime-grant suites passed uncached under `-race`; the full repository test suite, vet, build, formatting, checksum JSON and canonical OpenAPI generation checks passed. OpenAPI remains 166 operations and 145 schemas.

No real OpenAI call was made. The production adapter's fixed endpoint, remote response-storage disablement (`store:false`) and strict cited schemas were source-reviewed, while provider execution used an in-process deterministic synthetic boundary; this is not OpenAI privacy, availability or deployment acceptance. External MinIO/ClamAV/provider acceptance, independent security review, UAT and formal acceptance remain deferred. BID-BE-004 is `VERIFIED_TECHNICALLY`, not accepted. V7 verification follows below.

### V7 — BID-BE-005 review and exact-package approval — VERIFIED_TECHNICALLY 2026-10-23

PostgreSQL 15.19/TLS evidence traverses production control authorization, paid-write checks, application services and tenant stores for append-only review threads/comments, assignment history, dashboard/workload reads, immutable internal decision sequences, deterministic exact-package manifests and human approval. It proves one winner under concurrent comment edits; active same-tenant reviewer assignment; resolution without history mutation; `CHANGES_REQUESTED` followed by personal current `REVIEW_COMPLETE`; byte/hash-stable package replay; maker/approver separation; stale assurance denial; active-DENY precedence; one winner under concurrent approval; immutable approval replay/read; proposal-edit versus approval races; historical approval retention with current-applicability rechecks; bounded package/activity reads; actor provenance; and tenant-B non-observation.

Live execution exposed five bounded production defect classes. The control authorization persistence allowlist omitted the already-approved review and approval capabilities, making those operations unreachable. Review, decision, package and approval outbox inserts omitted mandatory `actor_id`, and three persistence call sites did not pass it. The raw readiness store built correct blockers but never set `Ready`, while package creation encoded an empty blocker slice as JSON `null`, violating the manifest's required-array constraint. Successful internal-review, package and approval HTTP responses also lacked private/no-store cache controls. These defects were corrected without changing approval policy; sqlc v1.31.1 output and reviewed query checksums were regenerated.

Authenticated HTTP evidence proves strict review, internal-decision, package and approval binding, including session-derived recent TOTP AAL2 data and private/no-store responses. Existing live MFA composition and Redis limit tests passed under `-race`, separately proving enrollment/login/step-up/replacement/replay behavior and fail-closed attempt limits. The selected V3–V7 PostgreSQL journeys passed together under `-race`; authorization, opportunity, HTTP, database, architecture and runtime-grant suites passed uncached under `-race`; full repository tests, vet, build, formatting, checksum JSON and canonical OpenAPI generation checks passed. OpenAPI remains 166 operations and 145 schemas.

This is synthetic technical verification, not deployment, independent security review, UAT or formal acceptance. Approval remains unsigned, unsubmitted and subject to current-applicability rechecks. BID-BE-005 is `VERIFIED_TECHNICALLY`, not accepted. V8 is next.

### V8 — BID-BE-006 external proof and submission confirmation — VERIFIED_TECHNICALLY 2026-10-23

PostgreSQL 15.19/TLS evidence traverses production control authorization, paid-write checks, application services, tenant persistence and document processing for immutable external-action reports, corrections, exact PDF proof upload and independent human confirmation. It proves package/bid linkage, preserved source timestamp text distinct from server recording time, report replay/body mismatch, foreign-bid report/proof denial, one active corrected report, the 25 MiB/PDF/ten-minute upload policy, exact object version/size/SHA-256 checks, QUARANTINED denial, exact CLEAN scan completion, current report/verify capabilities, active-DENY precedence, reporter/verifier separation, recent TOTP AAL2, one winner under concurrent confirmation, permanent `BID` to `SUBMITTED` uniqueness, exact committed lost-response replay after the state transition, conflicting replay denial, immutable actor-attributed audit/outbox evidence and tenant-B non-observation. The confirmation lineage retains the exact report, package, proposal approval and proof version, with reported, recorded and confirmed instants kept separate.

Live execution exposed bounded reachability and retry defects in already-approved functionality. The control background-processing allowlist and scan-commit capability validation omitted submission-report proof processing; the report/verify workflow capabilities were also missing from the workflow grant lookup. External-evidence/submission outbox writes lacked authenticated actors. Correction validation used `FOR UPDATE` against an immutable table despite the runtime role intentionally lacking UPDATE. Workflow idempotency conflicts and foreign report/proof mismatches surfaced as dependency failures, and confirmation checked post-transition BID readiness before exact replay, preventing lost-response recovery. Successful evidence/submission HTTP responses lacked private/no-store controls. These were corrected without broadening submission policy; sqlc v1.31.1 output and reviewed query checksums were regenerated.

Authenticated HTTP evidence proves strict report/confirmation binding, session-derived TOTP AAL2, private/no-store responses and fixed `bidora_transmitted=false`, `external_acceptance_confirmed=false` and `on_time_submission_confirmed=false` facts. The selected V3–V8 PostgreSQL journeys passed together under `-race`; opportunity, HTTP, authorization, database, architecture and runtime-grant packages passed uncached under `-race`; full repository tests, vet, build, formatting, checksum JSON and canonical OpenAPI generation checks passed. OpenAPI remains 166 operations and 145 schemas. One attempted all-live integration run failed because unrelated pilot/identity fixture permissions were not healthy; it is not counted as passing and the bounded V3–V8 journey rerun passed.

This is synthetic technical verification, not real MinIO/ClamAV deployment acceptance, independent security review, UAT or formal acceptance. Bidora records independently verified human evidence; it does not claim it transmitted the bid, that the buyer accepted it or that it was on time. BID-BE-006 is `VERIFIED_TECHNICALLY`, not accepted. V9 is next.

### V9 — BID-BE-007 outcomes and archive — VERIFIED_TECHNICALLY 2026-10-23

PostgreSQL 15.19/TLS evidence traverses production control authorization, paid-write checks, application services, tenant persistence and document processing for append-only reported outcomes, dedicated outcome proof and non-destructive archive. It proves proof-optional `PENDING`/`UNKNOWN`, terminal CLEAN-PDF proof requirements, QUARANTINED denial, exact object-version/hash/size processing, preserved reported-time text and distinct recorded time, expected sequence/current-leaf correction rules, original-history retention, one winner under concurrent terminal corrections, exact replay/body mismatch, bounded current/history reads, active-DENY precedence, actor-attributed audit/outbox evidence and tenant-B non-observation. Archive evidence covers submitted-workspace ineligibility without a confirmation, pending-upload/background-job and open-review blockers, eligible `NO_BID` closure, active-DENY precedence, one winner under concurrent archive, committed replay, immutable workspace enforcement, continued authorized historical reads and archived preparation state.

Live execution exposed bounded defects in the approved implementation. Outcome/archive capabilities were absent from the control workflow allowlist and outcome-proof processing was absent from the background and scan-commit allowlists. Outcome and archive outbox inserts omitted the authenticated actor. Proof and command idempotency conflicts surfaced as dependency errors, foreign/ineligible proof initiation was not mapped to a safe conflict, and the outcome outbox reused one parameter as incompatible integer/bigint types. Successful outcome, archive-preparation and archive HTTP responses lacked private/no-store controls. These were corrected without changing the outcome or archive policy; sqlc v1.31.1 output and reviewed query checksums were regenerated.

Authenticated HTTP evidence proves strict outcome/archive binding, private/no-store responses, `REPORTED_UNVERIFIED` provenance and fixed false buyer-authority, contract-award, withdrawal, analytics, deletion, legal-hold and restoration claims. The selected V3–V9 PostgreSQL journeys passed together under `-race`; opportunity, HTTP, authorization, database, architecture and runtime-grant suites passed uncached under `-race`; full repository tests, vet, build, formatting, checksum JSON and canonical OpenAPI generation checks passed. OpenAPI remains 166 operations and 145 schemas.

This is synthetic technical verification, not authoritative buyer/portal evidence, real MinIO/ClamAV deployment acceptance, retention disposition, independent security review, UAT or formal acceptance. Archive is immutable read-only closure, not deletion, legal-hold mutation or restoration support. BID-BE-007 is `VERIFIED_TECHNICALLY`, not accepted. V10 is next.

### V10 — Full backend journey and release gates — BLOCKED 2026-10-23

The first composed live gate ran all 35 PostgreSQL/Redis integration tests under `-race`. The V3–V9 application journeys and most identity/provider-free integration tests passed, but seven tests initially failed. A bounded assertion defect was corrected so identity registration now uses its migrator observer—not the intentionally INSERT-only runtime role—to inspect control audit evidence. Exact version-1 runtime grants were reconciled successfully on control and both tenant fixtures (68/61/61 tables), and stale synthetic tenant-schema projections in the full-pilot fixtures were advanced from common 5/Tenderpreneur 2 or 26 to the current common 9/Tenderpreneur 34. The identity composition rerun then passed.

Six full-pilot tests remain blocked at safe `pilot_deployment_open_failed`: bootstrap recovery, login/capture/logout, invitation composition, Google-start composition and two registration compositions. The current production `pilot.Build` requires the complete MinIO, OpenAI, ClamAV, document-measurer, PDF-text and TLS NATS JetStream configuration. The older recovery fixture does not provide that inventory, the HTTP fixture also predates document-measurement and NATS composition, and no authorized compatible TLS JetStream endpoint/credentials exist. A diagnostic-only partial fixture reached the exact safe `pilot_nats_config_invalid` boundary; those incomplete provider/process substitutions were removed rather than weakening production composition or fabricating release evidence. Creating a NATS fixture or making the bridge optional is outside this packet and requires a separately approved dependency/effect policy.

The repository also has no single composed account → document analysis → proposal → approval → submission → outcome → archive test. Passing the independently isolated V3–V9 journeys is retained evidence but is not relabeled as one crash/restart-safe end-to-end run. V10 therefore stopped before load/resource measurement, privacy/log inspection, current-schema restore-to-new-target, independent security assessment, provider acceptance or UAT. `go test ./... -count=1`, `go vet ./...`, `go build ./...` and canonical OpenAPI generation still passed after the bounded fixture corrections; these non-live checks do not override the failed live gate.

Exit target was not met. BID-BE-001–007 remain `VERIFIED_TECHNICALLY`, not `UAT_PENDING`; BID-QA-006 and BID-REL-005 remain Blocked. Only authorized human acceptance can mark application tickets `ACCEPTED`.

## Explicit non-goals

BID-BE-008–013, pricing, procurement publication/evaluation/award, exchange/sealed bidding, payment, platform administration, notification delivery, `/v1/me`, retention deletion/legal holds/restoration and broad AI search are not verification defects for Option A.
