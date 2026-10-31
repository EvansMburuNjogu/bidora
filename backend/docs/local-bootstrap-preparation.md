# Native bootstrap code preparation — BID-FND-004

> Historical evidence only. Tooling source/tests are now in [the retired archive](../../archive/local-fixture/README.md). Do not execute or resume these instructions by default.

Status: **VERIFIED_TECHNICALLY for the disconnected preparation slice; full bootstrap integration remains incomplete.** The user explicitly approved implementing the [bootstrap contract](native-postgres-bootstrap-review.md) without starting PostgreSQL. No cluster creation/start or web work is authorized by this increment.

## Implemented APIs

Package `bidora/backend/internal/localbootstrap`:

- `BuildPlan(localprofile.PostgresConfig) (Plan, error)` validates the local/test resource boundary and exactly `127.0.0.1:15432`, then returns freshly allocated fixed-role SQL steps. There is no arbitrary database/role mapping or fallback target.
- `GenerateCredential() (Credential, error)` generates an in-memory password from 32 cryptographically random bytes, a fresh 16-byte salt and a PostgreSQL-format SCRAM-SHA-256 verifier using standard-library PBKDF2/HMAC/SHA-256 with 4096 iterations. Password encoding is canonical ASCII base64url; no user-supplied Unicode password/SASLprep policy is introduced.
- `RestoreCredential(password []byte)` reconstructs a fresh-salt SCRAM verifier from an existing canonical private password without rotating/persisting it or advancing checkpoints; see [resume and resource-policy verification](native-resource-guard.md).
- `Credential.Password()` returns a fresh sensitive byte copy for the future private-file adapter. No file is written by this package.
- `Credential.ActivationSQL(role)` accepts only the six fixed migrator/runtime roles. It returns redacted `SecretSQL`; an explicit `Reveal()` is needed to obtain the verifier-bearing ALTER ROLE statement. No cleartext password is put in SQL. Bootstrap administrator/owners/provisioner and arbitrary names are rejected.
- `Diagnostics` implements a concurrent discard-only writer. It retains no raw process bytes, including partial lines, NULs or oversized output. `Snapshot`, formatting and JSON expose only synchronized saturated byte/write counts.

`Credential` and `SecretSQL` redact ordinary formatting and JSON, including nested values. Explicit password/SQL accessors intentionally cross that boundary; do not log their outputs. This is not a secret vault: Go strings/copies do not guarantee memory zeroization or protect against arbitrary same-process inspection.

## Plan semantics

The 16 steps comprise one transactional role/admin-access setup and five steps for each of the three fixed databases: nontransactional CREATE DATABASE, transactional database grants, two distinct credential activations, and transactional sentinel schema setup.

Roles begin NOLOGIN; owner/migrator/runtime flags, connection limits and membership match the approved contract. Credential steps have their own kind and no embedded SQL or secret, so an executor must not mistake empty statements for completed activation. The non-secret plan may be inspected without exposing generated passwords or verifiers.

Each step explicitly identifies its target database, authenticated role, effective role, transactional boundary, 5-second statement timeout and 1-second lock timeout. Future execution must implement those boundaries (including BEGIN/SET LOCAL ROLE/timeouts/COMMIT for transactional work, and session-scoped controls for CREATE DATABASE); metadata alone does not enforce them. The returned mutable data is not a trusted serialized command format. Never execute caller-edited/deserialized plans.

The only table is `bidora_fixture.sentinel`, with bounded synthetic key/value text. Runtime receives only schema USAGE and explicit SELECT/INSERT/UPDATE/DELETE on that table. No global future-table grants, business schemas, DROP/reset, implicit migration, default tenant, driver connection or API wiring is added.

## Initial-slice verification (historical)

For current combined bootstrap coverage and subsequent code verification, see [latest increment evidence](native-resource-guard.md).

On Go **1.27.2**, from `backend/`:

```sh
go test -count=1 -race ./...
go vet ./...
go build ./...
go test -count=1 -coverpkg=./internal/localbootstrap ./tests/localbootstrap
```

All passed. New package statement coverage: **95.3%**, not 100%. Untouched defensive paths include cryptographic entropy/derivation failures and unreachable-in-practical-tests saturated counter overflow; global entropy sources were not mutated to force coverage.

External suites in `tests/localbootstrap/` check:

- Exact role/database/sentinel SQL shapes, privilege flags, role separation, step ordering, explicit credential steps, transaction/deadline metadata and stable unique step IDs.
- Invalid environment/profile/resource/fixed-endpoint rejection; input preservation and no shared mutable plan state.
- Password/salt freshness, fixed-role activation restrictions, zero-credential rejection and safe errors.
- Independent PBKDF2 HMAC/XOR computation and SCRAM key/encoding comparison rather than reusing the production PBKDF2 helper.
- No cleartext password in activation SQL, redacted formatting/JSON and copy-safe password access.
- Raw-output canaries, oversized/partial content and concurrent diagnostic writes/snapshots/formatting/JSON under the race detector.

These are unit/structural checks, **not** PostgreSQL parsing, successful SCRAM login, role enforcement or cross-database isolation evidence. Real PostgreSQL tests remain pending and were not silently skipped or represented as passed. Existing health suites passed unchanged. No new dependencies, web actions or database processes were used.

## Remaining implementation and acceptance

The subsequent [private-store increment](local-bootstrap-store.md) now implements the metadata/credential ownership boundary and operation locking/SQL-plan checkpoints with centralized tests. The [resource decision guard](native-resource-guard.md) and resume-safe verifier reconstruction are now implemented and unit-verified. Native OS sampling, controlled process supervision, PostgreSQL initialization identity checks and the executor/reconciliation logic remain unimplemented. This original slice alone did not implement those components. The discard writer does not itself manage processes, drain deadlines, server logging configuration or stream lifetime.

Before live execution, preserve the explicit no-start boundary until separately authorized, and obtain applicable technical/FND-005 acceptance. Required real tests include successful/failed authentication, runtime privilege denial, cross-database refusal, partial-effect recovery and persistence across explicit stop/start. Independent/human acceptance and parent ticket gates remain open.
