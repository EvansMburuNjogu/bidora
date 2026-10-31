# Native fixture bootstrap contract — review draft

> Retired review draft, not current work authorization. See [the archive](../../archive/local-fixture/README.md); no bootstrap runner or PostgreSQL startup is authorized by the application-development cleanup.

**BID-FND-004 / APPROVED FOR IMPLEMENTATION, NOT LIVE EXECUTION.** The requesting user explicitly replied **“I approve”** to implementing this draft's roles, credentials, bootstrap SQL and safeguards **without starting PostgreSQL**. This document remains a non-executable contract, not a migration or permission to initialize/start a cluster. SQL placeholders below deliberately require substitution from a fixed reviewed mapping; never execute this document directly.

The requesting user approved the two [fixture-plan exceptions](native-postgres-fixture-plan.md) on 2026-10-17: loopback-only no-TLS synthetic connections with SCRAM authentication, and monitored native resource budgets with disclosed possible overshoot. The subsequent explicit implementation approval covers this role/credential/bootstrap contract; technical verification of the resulting code, FND-005 acceptance and accountable live-integration review remain separate.

## 1. Preconditions and fixed identities

Before any future mutating command: validate local/test scope, exact 15.19 isolated tools, experiment identity, new/owned private target, loopback endpoint, resource monitor, exclusive operator lock and bounded deadlines. Reject existing unrelated state or an occupied port. Never discover/adopt a default cluster.

| Database | NOLOGIN owner | Explicit migrator | Restricted runtime |
|---|---|---|---|
| `bidora_local_control` | `bidora_local_control_owner` | `bidora_local_control_migrator` | `bidora_local_control_runtime` |
| `bidora_local_tenant_a` | `bidora_local_tenant_a_owner` | `bidora_local_tenant_a_migrator` | `bidora_local_tenant_a_runtime` |
| `bidora_local_tenant_b` | `bidora_local_tenant_b_owner` | `bidora_local_tenant_b_migrator` | `bidora_local_tenant_b_runtime` |

Initial cluster administrator: `bidora_local_bootstrap`, created explicitly by initdb with a new private password file. Its superuser authority is confined operationally to this wholly owned fixture; it is never an application identity.

Proposed provisioner: `bidora_local_provisioner`, **NOLOGIN**, NOINHERIT, NOSUPERUSER, CREATEDB, CREATEROLE, NOREPLICATION, NOBYPASSRLS. Only the bootstrap administrator assumes it for explicit setup. PostgreSQL 15 CREATEROLE is powerful cluster-wide authority, not a per-database security boundary. Do not expose it to routine runtime checks or claim that revoking owner membership removes its ability to regrant non-superuser roles.

## 2. Cluster initialization and credential rules

Proposed initdb options: explicit new data directory, explicit bootstrap username, UTF8 encoding/C locale, `--auth-local=scram-sha-256`, `--auth-host=scram-sha-256`, and `--pwfile` pointing to a newly created private regular file. No `trust`, external socket directory, environment password or implicit current-user database selection.

Generate distinct fixture passwords from at least 32 cryptographically random bytes. Store only in exclusively created mode-0600 regular files beneath the private fixture root; reject links and pre-existing conflicting files. The initial administrator is the only initial login. Other roles start NOLOGIN with no password until their attributes/ownership/grants are verified.

Login activation must use the actual generated password's SCRAM verifier, not a shared test constant. Cleartext passwords never appear in SQL, process arguments, diagnostics or checkpoints. Verifiers are also sensitive. Do not put executable password-bearing SQL in repository files. Utility commands do not generally accept ordinary SQL bind parameters for PASSWORD: the implementation must explicitly review its verifier encoding/quoting path, not pretend `$1` works in arbitrary CREATE/ALTER ROLE syntax.

Recommended implementation path for review: derive a compatible SCRAM verifier with a fresh salt using the selected driver's reviewed facility or a separately tested standard-library implementation; allow only the expected verifier grammar and quote its SQL literal safely. No unreviewed crypto helper/export is added by this document. Prove successful and failed SCRAM authentication against real PostgreSQL before acceptance.

## 3. Proposed role creation and database setup

Execute role creation under explicit bootstrap/provisioner context after proving absence or matching prior owned checkpoint. For each fixed tuple:

```sql
CREATE ROLE <owner> NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE
  NOREPLICATION NOBYPASSRLS;
CREATE ROLE <migrator> NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE
  NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 1;
CREATE ROLE <runtime> NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE
  NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 2;
GRANT <owner> TO <migrator>;
GRANT <owner> TO bidora_local_provisioner;
```

No owner/migrator/provisioner/predefined privileged-role membership is granted to runtime. PostgreSQL 15 uses NOINHERIT plus explicit SET ROLE; do not copy version-16+ membership-option syntax into this bootstrap.

Database creation is a separate nontransactional operation under explicit provisioner context:

```sql
CREATE DATABASE <database> OWNER <owner> TEMPLATE template0;
```

Then explicit bootstrap-administrator grants in the same owned cluster:

```sql
REVOKE ALL ON DATABASE <database> FROM PUBLIC;
GRANT CONNECT ON DATABASE <database> TO <migrator>, <runtime>;
REVOKE <owner> FROM bidora_local_provisioner;
```

For the exclusively owned administrative databases, revoke PUBLIC CONNECT/TEMP on `postgres` and `template1`. Do not change `template0`'s disabled-connection property. The bootstrap administrator retains superuser access. No business tenant routing or production grants are implied.

## 4. Per-database fixture schema

Connect explicitly to each selected database as its migrator, after reviewed credential activation. Use explicit SET ROLE to the corresponding owner for the following transaction:

```sql
BEGIN;
SET LOCAL statement_timeout = '5s';
SET LOCAL lock_timeout = '1s';
SET LOCAL ROLE <owner>;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
CREATE SCHEMA bidora_fixture AUTHORIZATION <owner>;
REVOKE ALL ON SCHEMA bidora_fixture FROM PUBLIC;
CREATE TABLE bidora_fixture.sentinel (
    probe_key text PRIMARY KEY CHECK (octet_length(probe_key) BETWEEN 1 AND 64),
    probe_value text NOT NULL CHECK (octet_length(probe_value) <= 256)
);
REVOKE ALL ON TABLE bidora_fixture.sentinel FROM PUBLIC;
GRANT USAGE ON SCHEMA bidora_fixture TO <runtime>;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE bidora_fixture.sentinel TO <runtime>;
COMMIT;
```

No TRUNCATE, REFERENCES, TRIGGER, schema CREATE or database TEMP grant to runtime. No global default DML grants on future tables. No business tables, sequences, extensions, cross-database links or application audit/outbox migrations. This sentinel is only a fixture and does not inherit proposed business-schema columns or policies.

Before verification, inspect actual owners, role flags, memberships and ACLs; do not equate successful GRANT statements with least-privilege proof. Every runtime operation uses its exact database and credential, never the administrator.

## 5. Secret-safe server/process diagnostics

Proposed server logging is `stderr` with the logging collector disabled. The future supervising runner must own and continuously drain bounded stdout/stderr pipes, without persisting or echoing raw contents. Persist only allowlisted operation/error codes and counts; discard unrecognized text. This includes bootstrap/initdb diagnostics. Do not substitute `pg_ctl -l` to a raw log file.

Disable statement/duration/parameter logging; use terse errors and suppress error statements below PANIC. These settings alone are **not** a no-secret guarantee: PANIC/context/utility-command errors could contain verifiers. The no-raw-output boundary is therefore required even for failures and oversized/partial lines. SQL/driver logging must also be disabled. Synthetic canary tests must prove that plaintext passwords/verifiers do not reach committed evidence, checkpoints or diagnostic output.

Only the trusted administrator controls fixture server settings. Runtime has no superuser/predefined server-file privileges. This design is not a guarantee against the host user or a malicious bootstrap administrator reading private files/catalogs.

## 6. Retry, interruption and concurrency

Use one bounded fixture-operation lock. Record an owned intent/checkpoint before external effects and reconcile afterward. Role/schema work can use transactions within its database; CREATE DATABASE cannot. Never claim cross-database atomicity.

An ambiguous connection failure requires inspection of owned-cluster catalogs and the recorded intent, not blind CREATE/ALTER retries. Existing names alone do not establish ownership. Unexplained roles, grants, attributes, ownership, schema or credential drift stop execution for review; do not normalize an unrelated target with ALTER/DROP. Do not use `IF NOT EXISTS` as a substitute for verification. Preserve prior synthetic data and evidence on normal restart and failure.

## 7. Required tests before acceptance

- Real successful SCRAM login and wrong/missing credential rejection; runtime never connects via trust.
- Distinct sentinel DML in each selected database, with bounded parameterized values.
- Runtime denial of other synthetic databases plus `postgres`/`template1`.
- Runtime denial of CREATE DATABASE/ROLE/SCHEMA/TABLE/TEMP TABLE, TRUNCATE, SET ROLE to all privileged identities and attempts to grant itself membership.
- Migrator DDL succeeds only in its intended target; runtime remains a non-owner. Verify the exact creating role, not only the session login.
- Refuse wrong experiment identity, unexpected existing role/database/schema, changed ownership/grants, port conflict and concurrent setup.
- Interrupt before/after CREATE DATABASE and transactional schema commit; reconcile only proven owned state without destructive repair.
- Explicit stop/start preserves distinct synthetic sentinels; no ordinary startup/bootstrap side effects.
- Parent cancellation, connection/query/lock deadlines, resource-monitor failure/threshold stops and safe secret-free diagnostics.

Tests must remain under `backend/tests/`, with real PostgreSQL for accepted integration evidence. No frontend start or tests belong to this increment.

## Review disposition

The requesting user's explicit **“I approve”** authorizes implementation of this contract, not cluster startup or automatic acceptance of its code. The initial [disconnected code slice](local-bootstrap-preparation.md) is implemented and unit-verified: fixed SQL plans, generated in-memory SCRAM credentials and discard-only diagnostics. No SQL was executed, credential files persisted, database adapter wired, cluster created/started or business migration introduced. Filesystem/checkpoint/monitor/runner implementation and real integration acceptance remain pending. This approval is separately recorded from the earlier two local exceptions.
