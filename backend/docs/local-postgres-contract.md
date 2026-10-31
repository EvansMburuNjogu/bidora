# BID-FND-004 — Local PostgreSQL configuration and role contract

> Historical fixture contract, not current application configuration. Source/tests are in [the retired archive](../../archive/local-fixture/README.md). Do not execute or resume its tooling by default.

Status: **PROPOSED native-local contract / environment loader technically verified; database integration blocked**.

**Verified security blocker:** upstream advisories include installed PostgreSQL **15.17** in affected ranges; **15.19** is the current same-major candidate. Do not use 15.17 for live Bidora fixtures. See [native version/security evidence](native-postgres-security.md). The user subsequently authorized the isolated 15.19 source installation. [Build and tool-version verification passed](native-postgres-installation.md), without any cluster upgrade/start or system-tool replacement. The minimal build has no TLS; it does not authorize a runtime TLS exception or complete native resource/role/security acceptance.

**Current user direction supersedes the container proposal:** local development must not use Docker. Native PostgreSQL 15.17 (Homebrew) tools are present; their compatibility/security baseline still needs verification against the older 18.6 candidate. Do not install QEMU, start Colima, pull container images or alter existing clusters. The image/VM observations and section 4 container mechanics below are historical proposal/evidence, not current implementation prerequisites. Isolation, explicit setup, role separation, safe credentials and real-PostgreSQL tests remain required.

The next native implementation must use a separately owned synthetic data directory and explicit loopback endpoint, never a discovered/shared/default cluster. The approved 2 CPU / 2 GiB RAM / 10 GiB budget remains; native process/cluster limits need reviewed enforcement or monitoring and must not be claimed implemented by the configuration validator. `localprofile.Load` now parses explicit resource inputs without touching the environment, filesystem, services or API startup.

This is a reviewable design for the approved local-only foundation, not executable SQL or proof of a running database. Existing API startup remains `health-only`. No authentication/business endpoint, production policy or tenant registry is introduced.

## 1. Verified prerequisites and limits

On 2026-10-17 at 21:25 UTC, `docker info --format '{{.ServerVersion}}'` still failed because the configured local Docker daemon was unreachable. `docker compose version` returned 5.3.1. No runtime was launched, installed or substituted.

The approved experiment budget remains **2 CPUs, 2 GiB RAM, 10 GiB storage**. FND-005 validates requested ceilings but does not allocate resources. Its separate GPT-6 Astra read-only review returned `no_blocking_findings`; the reviewer did not rerun tests and did not grant user acceptance.

### Exact candidate evidence

Existing module/version and registry checks are recorded in [packet 13](../../docs/tickets/13_BACKEND_FOUNDATION_UNBLOCKING.md). New read-only license/advisory checks retrieved these exact upstream tag files successfully (HTTP 200):

| Component | License evidence | SHA-256 of retrieved license bytes |
|---|---|---|
| pgx v5.11.0 | [MIT](https://raw.githubusercontent.com/jackc/pgx/v5.11.0/LICENSE) | `467f95e074fe23079a5623ed652619682692041b8551da27e3c2ddb9659a1507` |
| Goose v3.29.0 | [MIT](https://raw.githubusercontent.com/pressly/goose/v3.29.0/LICENSE) | `f23985556a879c0e68bb4e136ee4b88a550d26fa33db77a65d76b721c93093ef` |
| sqlc v1.31.1 | [MIT](https://raw.githubusercontent.com/sqlc-dev/sqlc/v1.31.1/LICENSE) | `fc69a4525f6f7e79675fba3645351f2cdaf9331978e72d44155634575a4455ea` |
| PostgreSQL 18.6 | [PostgreSQL license](https://raw.githubusercontent.com/postgres/postgres/REL_18_6/COPYRIGHT) | `3d6af92ff8a4c2cdf69afb1cf44edea727922f5cd0cf8b5f72b11cdecac8fdfd` |

The MIT texts require preserving copyright and permission notices; PostgreSQL requires preserving its specified notice and disclaimer paragraphs. This identifies direct upstream terms, not legal certification or a transitive/image license inventory.

POST queries to `https://api.osv.dev/v1/query` with ecosystem `Go` and the exact module/version pairs `github.com/jackc/pgx/v5@v5.11.0`, `github.com/pressly/goose/v3@v3.29.0`, and `github.com/sqlc-dev/sqlc@v1.31.1` returned **no advisory IDs** at query time. These are direct-version database lookups, not a resolved dependency scan, call-graph analysis or a finding-free image scan. Do not infer a blanket security approval.

Proposed image remains official `postgres:18.6` pinned to verified Linux/amd64 manifest digest `sha256:885953109528ad3dfc90362b1a6f50a78620b5315be19f187753d267e484dc5b`. Digest evidence is not signature/provenance validation or host execution. Base-image packages, transitive modules and actual build/platform compatibility still require review. Goose/sqlc execution can wait for the migration/query-generation increment; do not install unused tools merely to start a PostgreSQL fixture.

## 2. Profile and configuration boundary

`backend/cmd/local-check` now provides a runnable **configuration-only** preflight. It validates the explicit resource settings and loopback endpoint without I/O to PostgreSQL. Fixture bootstrap and live connection checks remain proposed, not implemented. `cmd/api` still rejects `postgres-local`; it never silently connects from the health profile.

| Proposed input | Rule |
|---|---|
| Environment/profile | Explicit `local` or `test`, and `postgres-local`; validated by the FND-005 helper |
| CPU/memory/storage request | Explicit integer units; normal experiment requests exactly 2 / 2048 MiB / 10 GiB; no defaults or clamping |
| PostgreSQL endpoint | Literal loopback address, explicit valid port; reject public binds, host lists, arbitrary URL query parameters and remote endpoints |
| Database targets | Fixed synthetic `bidora_local_control`, `bidora_local_tenant_a`, `bidora_local_tenant_b`; duplicate/missing/unknown targets fail, with no fallback database |
| Credentials | Role-specific local secret-file references; no inline passwords in arguments, committed files, logs or errors |
| Target confirmation | Explicit experiment identity and selected operation; bootstrap never runs implicitly on ordinary start |

An absent runtime, role credential or required database fails the fixture check. Error messages identify safe field/operation codes, not DSNs or secret contents. Any local TLS exception must be explicit, restricted to this isolated loopback fixture, and impossible in non-local configuration; production transport policy remains unselected.

Use reviewed pgx connection construction with allowlisted fields rather than allowing libpq environment variables/connection-string options to silently redirect connections. Bound connection/query deadlines and close every test pool. This fixture mapping is not the future server-authorized tenant router.

## 3. Role/ownership matrix

For each of the three databases, create a distinct owner, migrator and runtime role. No shared runtime identity across tenants.

| Role | Intended authority | Never used for |
|---|---|---|
| Bootstrap administrator | Initial PostgreSQL cluster setup and explicit reviewed role/grant creation on the owned disposable cluster only | API, readiness or routine integration-test queries |
| Local provisioner | Explicit target creation under reviewed CREATEDB/CREATEROLE authority; never exposed through HTTP | Application DML, normal start or automatic cleanup |
| Per-database owner | NOLOGIN; owns its database and dedicated fixture schema | Application authentication |
| Per-database migrator | Login permitted only for explicit setup/migration operations; may SET ROLE only to its intended owner under reviewed grants | API runtime or tenant-wide default credentials |
| Per-database runtime | NOSUPERUSER, NOCREATEDB, NOCREATEROLE, NOREPLICATION, NOBYPASSRLS; only its database CONNECT, fixture schema USAGE and explicitly granted fixture-table DML | DDL, role creation/membership changes, extensions or cross-database access |

Revoke PUBLIC CONNECT/TEMP on the synthetic databases, and PUBLIC CREATE on relevant schemas. On this exclusively owned cluster, also review default access to `postgres` and `template1`; do not claim cross-database denial while default PUBLIC connection grants remain. Verify owner/migrator membership cannot be inherited or assumed by runtime roles. Do not modify permissions on any shared/pre-existing server.

The migrator creates only a synthetic sentinel fixture with explicit grants to that database's runtime. Do not globally grant CRUD on future business tables: audit/evidence immutability and module-specific privileges belong to reviewed business migrations. Verify default privileges for the **actual creating role**, not merely the login used to execute setup.

PostgreSQL bootstrap may initialize cluster files on an empty volume. Application database/role setup is a separate explicit operation, not an image init-directory script or API startup side effect. Retrying a partial setup must inspect existing ownership and fail on conflicts; never adopt unrelated resources or fix conflicts with DROP.

## 4. Historical container storage proposal — not the native-local implementation

- One isolated PostgreSQL service; no broker, cache, email, storage/scanner or AI services.
- Loopback-only published port, fixed accepted image digest, no privileged container or Docker socket mount.
- CPU/memory limits must be verified from actual engine settings and container inspection. Document OOM behavior rather than assuming a successful Compose parse proves enforcement.
- Dedicated persistent data volume; normal stop/start preserves it. No `down -v`, destructive reset or automatic seed/migration command.
- **Storage blocker:** a standard named volume is not a 10 GiB quota. Before provisioning, choose and review an enforceable storage mechanism or an explicit monitored-budget policy. Monitoring must include database/WAL, container logs and image/storage overhead; checks cannot guarantee no overshoot. Stop before allocation if the approved budget cannot be respected. Do not silently redefine the budget as database-table size only.
- Bound log rotation and safe diagnostics. Resource exhaustion stops the test; it does not authorize raising approved limits.

## 5. Integration acceptance matrix

All Go tests remain external packages under `backend/tests/`. Use actual PostgreSQL, not success fakes.

| Case | Required observation |
|---|---|
| Existing health-only start | No PostgreSQL dependency introduced; existing health tests still pass |
| Missing/invalid settings or non-local target | Reject before connecting, with safe errors |
| Missing engine/unavailable endpoint | Clear failure, not successful integration or a hidden skip |
| Correct runtime role | Only its intended database and granted fixture table work |
| Cross-database attempt | Each runtime credential denied the other two databases and unintended administrative database access |
| Privilege escalation | CREATE TABLE/SCHEMA/DATABASE/ROLE, SET ROLE to owner/migrator and temp-table creation denied as applicable |
| Explicit migrator | DDL succeeds only within its reviewed scope; runtime remains non-owner |
| Persistence | Write distinct synthetic sentinel values, ordinary stop/start, reconnect and verify values unchanged |
| Partial bootstrap retry | Same owned targets reconciled without destructive recreation; ownership mismatch fails closed |
| Port/secret/resource inspection | Loopback binding and expected limits visible; no secret values captured in committed evidence |

Connectivity and sentinel separation do not prove future application authorization, revocation, SQL injection resistance or full tenant isolation. FND-004 remains Blocked until reviewed pins/configuration/roles, accepted FND-005 evidence, a usable engine, storage-budget treatment and actual tests are available. No production readiness or independent second-engineer reproduction is claimed here.

## 6. Recovery and next operation

The original contract preparation was documentation only. Under subsequent explicit user approval, a separate `bidora` Colima profile was configured and startup attempted; it failed because `qemu-system-x86_64` is missing. No PostgreSQL/container service or credentials were created. Profile/VM metadata, a sparse 4 GiB-capacity data disk and a cached VM image remain; preserve these rather than deleting unrelated runtime state.

The new profile requests 2 CPUs / 2 GiB RAM, 4 GiB root + 4 GiB data capacity, leaving 2 GiB budget headroom for cache/metadata. At failure, the data disk used 196 KiB on the host and the shared Colima cache measured 349,904 KiB; the root disk was not demonstrated allocated/running. No home mounts, SSH agent forwarding or active-context change was requested. The original default profile remains stopped and unchanged. Detailed commands/evidence are in [FND-004](../../docs/tickets/foundation/BID-FND-004.md).

The user has since rejected Docker for local development. Do not continue the QEMU/Colima path. Leave its stopped state untouched. Next steps are native PostgreSQL version/security verification and review of the isolated cluster/role/resource contract, followed by explicitly scoped bootstrap/integration code. No identity or business schema work is authorized. The disconnected environment loader and unit tests already provide a Docker-free code increment without waiting for host setup.
