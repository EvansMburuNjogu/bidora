# Local resource configuration preparation — BID-FND-005 / BID-FND-004

> Historical evidence only. Tooling source/tests are now in [the retired archive](../../archive/local-fixture/README.md). The local-check command is no longer in the active backend; do not execute or resume these instructions.

`internal/localprofile.Config` is a pure standard-library validation boundary for future local PostgreSQL integration. It and the separate `localprofile.Load` environment parser are **not wired into API startup or readiness**. The user's current local-development direction is native PostgreSQL, without Docker or Colima. Existing startup still requires `BIDORA_PROFILE=health-only`; setting `postgres-local` on the current API does not enable a new service.

## Usage within the backend module

```go
cfg := localprofile.Config{
    Environment: "local",
    Profile:     "postgres-local",
    CPUs:        2,
    MemoryMiB:   2048,
    StorageGiB:  10,
}
if err := cfg.Validate(); err != nil {
    // Stop before future provisioning/configuration use.
    return err
}
```

Import `bidora/backend/internal/localprofile` from an allowed backend package.

| Field | Accepted values |
|---|---|
| Environment | Exactly `local` or `test` |
| Profile | Exactly `postgres-local` |
| CPUs | Integer 1–2 inclusive |
| MemoryMiB | Integer 1–2048 inclusive |
| StorageGiB | Integer 1–10 inclusive |

All values are explicit: no defaults, trimming, case normalization or clamping. Zero/negative resources, excessive requests and other environments/profiles fail. Integer fields cannot represent fractional settings. FND-005 originally introduced only this value validator; the environment parser below is a subsequent FND-004 preparation increment, not part of the earlier source-review verdict. Validation returns the first invalid field in declaration order using a static error message without echoing input. It does not mutate its receiver or perform I/O.

## Explicit environment loading (native or test tooling)

`localprofile.Load(getenv func(string) string) (Config, error)` accepts an injected environment reader. Future native tooling can pass `os.Getenv`; tests supply a map. The package never reads the ambient process environment itself.

Required keys:

```text
BIDORA_ENV=local
BIDORA_PROFILE=postgres-local
BIDORA_LOCAL_CPUS=2
BIDORA_LOCAL_MEMORY_MIB=2048
BIDORA_LOCAL_STORAGE_GIB=10
```

These are **not** settings to use with the current health API. They are inputs to the disconnected local-profile loader only. `test` is also an accepted environment.

Numeric inputs must be canonical positive ASCII decimal integers: no signs, leading zeros, whitespace, fractional/exponential notation, unit suffixes or Unicode digits. Missing, malformed, overflowed or over-budget values return an error and an empty configuration. A nil reader fails safely. Parsing errors name only the fixed field, never its supplied value. Successful parsing delegates final scope/resource checking to `Config.Validate`; it does not start PostgreSQL or allocate host resources.

## Runnable native configuration check

From `backend/`:

```sh
BIDORA_ENV=local BIDORA_PROFILE=postgres-local \
BIDORA_LOCAL_CPUS=2 BIDORA_LOCAL_MEMORY_MIB=2048 BIDORA_LOCAL_STORAGE_GIB=10 \
BIDORA_PG_ADDR=127.0.0.1:15432 go run ./cmd/local-check
```

Successful output explicitly says that connectivity, credentials, roles and resource enforcement are **NOT checked**. The compiled command returns 0 for valid configuration and 1 for invalid configuration or output failure. It never connects to the specified port, starts a service or initializes a cluster.

`localprofile.LoadPostgres` composes the resource loader with required `BIDORA_PG_ADDR` validation. Only literal loopback IPv4 or bracketed IPv6 and explicit ports 1–65535 are accepted. Hostnames, remote/wildcard addresses, IPv4-mapped IPv6, scoped IPv6, URLs/DSNs, host lists, query parameters, signs/leading zeros/whitespace in ports and missing values fail with a static error. No DNS or libpq environment fallback occurs. The result has no credentials or default database target.

This entry point is separate from `cmd/api`: it is not a database readiness endpoint. Unit tests live under `tests/localprofile/` and `tests/localcheck/`; test commands remain below.

## Explicit native toolchain inspection

The default command above still performs no subprocess execution. Opt in to inspecting an **operator-trusted** native installation:

```sh
BIDORA_ENV=local BIDORA_PROFILE=postgres-local \
BIDORA_LOCAL_CPUS=2 BIDORA_LOCAL_MEMORY_MIB=2048 BIDORA_LOCAL_STORAGE_GIB=10 \
BIDORA_PG_ADDR=127.0.0.1:15432 \
BIDORA_PG_BIN_DIR="$HOME/.local/share/bidora/postgresql/15.19/install/bin" BIDORA_PG_VERSION=15.19 \
go run ./cmd/local-check --tools
```

The user approved an isolated native **15.19** installation, and its source build/tool-version checks passed; see [installation evidence and limitations](native-postgres-installation.md). This minimal build has **no TLS support** and is not yet approved for database connections. System **15.17** remains untouched and affected by upstream advisories; do not use it for Bidora integration. See [native security evidence](native-postgres-security.md). Supply the explicit expected version for your separately reviewed installation. The directory must be a clean absolute path; no PATH lookup or default installation is adopted. Symlinks in a trusted installation are permitted. Four tools (`postgres`, `pg_ctl`, `initdb`, `psql`) must all report that exact version using `--version`. Missing, failing, inconsistent, malformed or excessively verbose tools fail closed. Unknown command arguments return exit 2 before reading configuration.

Inspection uses sequential commands, 5-second individual and 15-second overall deadlines, a 250 ms pipe-drain limit and a 4096-byte combined stdout/stderr cap per command. Parent cancellation is preserved; the CLI handles interrupt/termination signals. The child environment contains only fixed locale and system PATH settings—not inherited provider credentials, libpq settings or dynamic-loader overrides. Safe diagnostics retain wrapped causes for trusted Go error inspection without printing raw paths/process output.

**This is not a sandbox or a version/security endorsement.** Only run it against trusted installed executables; `--version` cannot make arbitrary binaries harmless. Cancellation targets the selected process, not an arbitrary descendant process tree. The inspected binaries retain host-user permissions. No cluster initialization, service startup, database connection, credential generation or upgrade is performed by this command.

External tests in `tests/localpostgres/` use synthetic shell executables to exercise negative cases and cancellation; they are not PostgreSQL integration evidence. Separate historical and current real invocations matched the old system 15.17 tools and the newly isolated 15.19 tools respectively; the latter is now the explicit diagnostic target. Unix shell fixture tests skip on Windows; this host run had no such skips.

## What success does not mean

- No resources have been allocated, and Docker/VM settings have not been inspected or changed.
- Smaller positive values pass the ceiling check but are **not** established as viable PostgreSQL sizing. The approved experiment budget is 2 CPUs, 2 GiB RAM and 10 GiB storage.
- A 10 GiB configuration value does not impose a filesystem quota. Ordinary Compose volumes do not automatically enforce that budget.
- No database connectivity, isolation, role privileges, readiness, license/security acceptance or production fitness is established.

[BID-FND-004](../../docs/tickets/foundation/BID-FND-004.md) owns later reviewed integration and real service tests. Its former Docker prerequisite is superseded by the user's native-local choice. Native PostgreSQL version compatibility/security, isolated cluster configuration, roles and resource-budget treatment still require verification before integration. This helper must not bypass those gates or introduce automatic migrations/seeding/reset.

## Verification and recovery

From `backend/`:

```sh
go test -count=1 -race ./...
go vet ./...
go build ./...
```

External tests in `tests/localprofile/` cover valid boundaries, invalid scope/resources, extreme integers, multiple invalid fields, input preservation and safe errors. `load_test.go` also covers explicit/missing fields, malformed/overflowed inputs, safe errors, nil readers, ambient-environment independence and the unchanged health-only startup boundary. No Docker or new dependencies are required. Existing configuration/HTTP suites remain the health regression checks.

There are no persisted effects to roll back. The package is disconnected preparation; future changes must keep that boundary until integration is explicitly reviewed.
