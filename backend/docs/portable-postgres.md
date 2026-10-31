# Portable Go/PostgreSQL connection layer

**Status: VERIFIED_TECHNICALLY for configuration, lifecycle and negative-protocol tests; real database integration and deployment acceptance remain pending.**

The user redirected work from local-fixture tooling to server-suitable Go/PostgreSQL code, then explicitly said “proceed”. This scoped increment implements connection management only. It does not authorize server provisioning, production deployment, business schemas/APIs, database startup or web work. Related foundation tickets remain open.

## Cleanup update

The local-tooling source/tests are now preserved in [the retired archive](../../archive/local-fixture/README.md), outside the active module. The verification below records the pre-cleanup adapter increment, not current execution of those retired suites. See [current application-development checks](application-development.md). The portable adapter and health-only API remain active and unchanged by the archive move.

## Code and boundaries

- `internal/postgres/config.go`: explicit validated connection/pool settings and private pgx configuration construction.
- `internal/postgres/pool.go`: lazy owned pool, deadline-bounded Ping, bounded transaction callbacks and cancellation-aware shutdown.
- `tests/postgres/pool_test.go`: centralized external-package validation, lifecycle and synthetic negative-wire tests.
- `go.mod` / `go.sum`: pinned pgx and resolved dependencies.

This package has no dependency on the local fixture/store/monitor packages or their paths. It imports neither Fiber nor frontend code. It is **not wired into `cmd/api`**: the existing health-only behavior still rejects a PostgreSQL profile and certifies no database readiness.

`postgres.New(Config)` creates a lazy pool with zero minimum connections, not a successful connection. `Ping(ctx)` explicitly checks the selected target and role. `WithinTransaction` owns read-write begin/rollback/commit and `WithinReadOnlyTransaction` provides the same bounded lifecycle with PostgreSQL read-only access; both give infrastructure adapters a context bounded by caller cancellation, pool lifecycle and operation timeout; callbacks must contain database work only. Commit failure is reported as ambiguous failure, never success. `Close(ctx)` stops new work, cancels active operations and waits for cleanup. If the caller deadline expires, it reports incomplete shutdown; cleanup continues and a subsequent Close can wait again. Raw pgx pools/connections are not exported, preventing uncontrolled borrowed connections from defeating this lifecycle.

No general query API, migration runner, tenant router, implicit retry/failover, credential generation or readiness HTTP handler is added. Those require their applicable contracts and tests. A database name or Config is not authorization: callers must resolve targets/credentials from trusted server configuration or validated membership/registry context, never directly from a request header.

## Explicit configuration and secure transport

All settings are required; no endpoint/database/user/password or capacity fallback is supplied:

- One DNS hostname or unbracketed IP and explicit port. Reject URL/DSN/host lists, Unix-socket paths, scoped addresses, wildcard IPs and malformed hosts.
- Explicit ASCII database/role identifiers (letters/underscore, followed by letters/digits/underscore, at most 63 bytes). Arbitrary quoted PostgreSQL names are intentionally outside this initial boundary.
- Explicit nonempty bounded password; nil/empty trust roots are rejected. The operator supplies a populated `x509.CertPool`; it is cloned to avoid later caller mutation changing the pool's trust policy.
- TLS certificate-chain **and hostname/IP verification**, minimum TLS 1.2; no insecure skip flag, downgrade or plaintext fallback.
- SCRAM-SHA-256 authentication required; cleartext/MD5/trust authentication is not silently accepted. Client-certificate-only/OAuth/GSS authentication is not implemented.
- Explicit maximum pool size 1–32; connect/operation timeouts from 1 ms to 30 s; positive lock timeout no greater than operation timeout. Explicit connection lifetime, idle time and health-check period are validated and applied. These are component safety bounds, not approved production capacity/SLOs or fleet-wide pool budgets.
- Session defaults: application name `bidora`, UTC, `search_path=pg_catalog`, bounded statement/lock/idle-transaction timeouts. Future SQL must qualify reviewed schemas. Protocol message bodies are limited to 16 MiB.

The local fixture's approved no-TLS exception **does not apply here**. The separately installed minimal native 15.19 build has no TLS and cannot be used for successful tests of this adapter without separately reviewed TLS-capable PostgreSQL setup. No bypass was added to make a test green.

## Ambient settings and secret handling

The adapter rejects nonempty `PG*` environment settings before driver parsing; use explicit typed configuration instead. It does not mutate global environment variables. Treat the application environment as immutable after startup.

pgx requires configs produced by its parser. The adapter parses a fixed inert template with placeholder credentials, disabled parse-time TLS, explicit null-device service/pass files and empty certificate paths, then replaces endpoint/auth/TLS/session/pool settings before creating a pool. Actual secrets are never interpolated into a DSN. The template prevents sensitive home-file/service redirection if another goroutine changes PG settings during construction; such changes may cause failure, not permission to redirect. No real connection is created with the inert template.

Config formatting/JSON and operational errors are redacted. Wrapped causes are preserved for trusted `errors.Is`/`errors.As` inspection; do not log unwrapped driver errors or passwords. No driver tracer, raw SQL logger or external credential provider is installed. This does not protect against arbitrary same-process/host-memory access.

## Dependency verification

Pinned/resolved versions:

| Module | Version | Inspected license |
|---|---|---|
| `github.com/jackc/pgx/v5` | `v5.11.0` | MIT, copyright Jack Christensen 2013–2021 |
| `github.com/jackc/pgpassfile` | `v1.0.0` | MIT, copyright Jack Christensen 2019 |
| `github.com/jackc/pgservicefile` | `v0.0.0-20240606120523-5a60cdf6a761` | MIT, copyright Jack Christensen 2020 |
| `github.com/jackc/puddle/v2` | `v2.2.2` | MIT, copyright Jack Christensen 2018 |
| `golang.org/x/sync` | `v0.24.0` | BSD-3-Clause, copyright The Go Authors 2009 |

Existing module versions were retained; x/sync v0.24.0 is the resolved module-graph selection recorded explicitly by tidy, not an assumed pgx-minimum pin. pgx declares Go >=1.25.0; actual compilation/tests use **Go 1.27.2**. pgx source tag resolves to commit `5e583fa7aabfa88b796292f849fc9d7d75ac159d`; downloaded module sum is `h1:IzBBtyK9AHqf98cctWFifYSci2hgQR/cd56wB4p+ogg=`. `go mod verify` passed for all modules.

License texts in the downloaded module sources were read; their copyright/permission/disclaimer notices must accompany any applicable redistribution. BSD terms also prohibit endorsement using contributor names without permission. This records dependency terms/integrity, not legal certification, a complete distributed-artifact notice bundle or independent source-signature verification.

## Actually executed checks

From `backend/`:

```sh
go mod download -json github.com/jackc/pgx/v5@v5.11.0
go get github.com/jackc/pgx/v5/pgxpool@v5.11.0
go mod tidy
go mod verify
GOPROXY=off go test -count=1 -race ./...
GOPROXY=off go vet ./...
GOPROXY=off go build ./...
GOPROXY=off go test -count=1 -coverpkg=./internal/postgres ./tests/postgres
```

All succeeded. Adapter statement coverage: **94.1%**, not 100%. Existing health and backend suites passed. The paused storage collector happens to compile as part of the module but has no dedicated test suite and remains unwired/unverified; this increment does not complete it.

Negative-wire tests use temporary loopback TCP/TLS peers, **not a PostgreSQL server**. They verify the PostgreSQL TLS request, rejection of an untrusted certificate even after the caller mutates its original CA pool, explicit startup database/role values, and rejection of a cleartext-password challenge over verified TLS without sending a password. Lifecycle tests cover lazy creation, caller cancellation, blocked work cancellation on close and closed/zero pool refusal. Validation/redaction tests reject missing/unsafe settings and ambient PG overrides before parser use.

Linux/amd64 adapter tests cross-compiled with `CGO_ENABLED=0`; Linux runtime tests were not executed. Temporary binaries were removed. The pinned cached govulncheck v1.9.0 was built/run using the verified Go 1.27.2 compiler and PATH: verbose scan covered **22 modules plus go1.27.2**, with **0 reachable-symbol findings and 0 imported-package findings**. Existing module-only **GO-2026-5932** remains for unused `golang.org/x/crypto/openpgp` v0.58.0, no fixed version. No findings were suppressed; the advisory database was fetched over the network.

## Remaining gates

Real PostgreSQL tests must prove successful verified-TLS/SCRAM connection, connection/pool limits, unavailable/wrong-credential behavior, session settings, transaction cancellation and shutdown against established connections. Synthetic rejection tests are not that evidence.

Before deployment: approve server runtime/security/secret management, supported platform/DB build, fleet/pool budgets, backups/restores, observability, network policy and operational acceptance. No business operation/schema approval is inferred. Local-only bootstrap/collector work is retired and archived; retain its artifacts without executing/restoring them by default. Web remains untouched and unstarted.
