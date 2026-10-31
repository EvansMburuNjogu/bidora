# BID-ARC-002 — Repository and toolchain decision packet

Prepared: 2026-10-17. Status: **Repository/tool bundle approved; exact version acceptance and compatibility/security verification remain pending**.

Ticket: [BID-ARC-002](architecture/BID-ARC-002.md). This packet advances independently of ARC-001's remaining reviewer evidence. The requesting user explicitly accepted the recommended repository/tool bundle: **“apporve and proceed”** (recorded 2026-10-17). This approves `web/`, `backend/`, Goose and npm. It does not certify candidate versions or authorize scaffolding of blocked foundation tickets.

## 1. Recommended decision bundle

| Decision | Recommendation | Alternative / consequence | Status |
|---|---|---|---|
| Frontend root, BID-ADR-005 | Keep existing `web/`; retain `backend/` and `docs/product/` | Rename to `frontend/` before scaffolding if preferred; would require coordinated reference changes. Never keep both apps. | Approved by requesting user |
| Migration tool, BID-ADR-008 | Goose, SQL-first migrations; explicit pgx transactions and sqlc | golang-migrate is viable; mixing migration authorities is not. Goose fits existing SQL-first design but still needs proof of required rollout semantics. | Approved tool; proof pending |
| JavaScript package manager | npm with one committed `web/package-lock.json`; CI uses lockfile-based installation | pnpm offers workspace features not currently needed by one web app; no second lockfile | Approved package manager; exact pin pending |
| Version baseline | Exact candidates in §3; gate them on compatibility/security proof and missing Node release evidence | Never install unreviewed `latest` or infer supported versions from local availability | Proposed, not a validated build |
| Repository conventions | Ticket-scoped branches, reviewed changes, protected main once Git/hosting is separately authorized | No host chosen or Git initialized by this packet | Proposed |
| Dependency boundaries | Existing domain/application/adapter direction; contracts before consumers | No ORM, cross-module repository access or frontend DB queries | Existing approved architecture, clarified here |

The accepted bundle selects paths and tools, not passing tests or approved providers/identity/business rules. BID-ADR-005/008, root AGENTS and the repository/implementation guidance now reflect this approval. Exact candidate pins remain subject to the explicit verification gates below.

## 2. Actual workspace and host inspection

Read-only inspection on 2026-10-17:

| Check | Observed result | Interpretation |
|---|---|---|
| `find backend web -maxdepth 2 -type f` | No files | Empty application roots; no existing app to migrate |
| `node --version` | `v24.16.0` | Installed runtime, not independently verified support/patch status |
| `npm --version` | `11.13.0` | Installed CLI; not yet a project package-manager pin |
| `go version` | `go1.27.1 darwin/amd64` | Installed toolchain differs from proposed patch candidate |
| `docker --version` | `29.6.2`, build `dfc4efb1e2` | CLI exists; daemon health, Compose and Testcontainers compatibility not tested |
| `git status --short` | Fatal: not a git repository; exit 128 | Expected baseline limitation; Git was not initialized |

No framework generators, package downloads/install commands, builds, database connections, image pulls or migrations ran. Compatibility research used read-only HTTPS requests to documentation and package metadata endpoints.

## 3. Exact candidate register and compatibility evidence

These versions were observed from the stated sources during this review. They are **candidates**, not security-reviewed or experimentally validated pins. Moving metadata endpoints must be rechecked at authorized kickoff; exact versioned metadata links identify the candidate more durably.

| Component | Candidate | Evidence observed | Future pin location / remaining proof |
|---|---|---|---|
| Go | `1.27.2` | [Official release metadata](https://go.dev/dl/?mode=json) returned stable `go1.27.2` and `go1.26.9` | `backend/go.mod` toolchain policy plus CI/container toolchain pin; clean compile/tests required. Local `1.27.1` is not this pin. |
| Fiber | `v3.6.1` | [Versioned module file](https://proxy.golang.org/github.com/gofiber/fiber/v3/@v/v3.6.1.mod) declares `go 1.26.0` | `backend/go.mod` / `go.sum`; Go candidate satisfies declared minimum, but actual HTTP/middleware tests remain |
| pgx | `v5.11.0` | [Versioned module file](https://proxy.golang.org/github.com/jackc/pgx/v5/@v/v5.11.0.mod) declares `go 1.25.0` | `backend/go.mod` / `go.sum`; PostgreSQL integration and transaction tests required |
| Goose | `v3.29.0` | [Versioned module file](https://proxy.golang.org/github.com/pressly/goose/v3/@v/v3.29.0.mod) declares `go 1.26.0` | Explicit tool/binary version and checksum in future build tooling; migration proof required |
| sqlc | `v1.31.1` | [Versioned module file](https://proxy.golang.org/github.com/sqlc-dev/sqlc/@v/v1.31.1.mod) declares `go 1.26.0`, `toolchain go1.26.2` | Future tool manifest/CI pin; pgx/v5 generation and compile/drift checks required |
| PostgreSQL | `18.6` | [Official version policy](https://www.postgresql.org/support/versioning/) lists 18.6 as supported, final support date November 14, 2030 | Future Compose image tag **and verified platform digest**; digest not obtained. Provider availability, extension compatibility and real tests remain. |
| Node.js | `24.22.0` revised candidate | Retried [official release metadata](https://nodejs.org/dist/index.json) successfully: latest listed 24.x release `v24.22.0`, dated 2026-10-14, LTS Krypton. [Official lifecycle schedule](https://raw.githubusercontent.com/nodejs/Release/main/schedule.json) lists maintenance from 2026-10-20 and end-of-life 2028-04-30. | Future `web/.node-version`, package engines and CI/container pin; local 24.16.0 is older. Security/advisory and build review still required. |
| npm | `11.19.1` revised candidate | Node 24.22.0 release metadata lists npm 11.19.1; [versioned registry metadata](https://registry.npmjs.org/npm/11.19.1) requires Node `^20.17.0 || >=22.9.0` | Future `web/package.json` packageManager plus CI pin; Node 24.22.0 satisfies declared engine range. Local npm 11.13.0 remains unchanged. |
| Next.js | `16.4.2` | [Registry metadata](https://registry.npmjs.org/next/16.4.2): Node `>=20.9.0`; React peer includes `^19.0.0`. [Official installation requirements](https://nextjs.org/docs/app/getting-started/installation) also state Node minimum 20.9 | Exact direct version and lockfile; App Router, SSR privacy, build, lint and health-API smoke proof |
| React / React DOM | `19.3.1` / `19.3.1` | [React](https://registry.npmjs.org/react/19.3.1) and [React DOM](https://registry.npmjs.org/react-dom/19.3.1) metadata; DOM peer React `^19.3.1` | Exact matching direct versions and lockfile; satisfies Next peer range, not proof of all component compatibility |
| TypeScript | `7.0.2` candidate only | [Registry metadata](https://registry.npmjs.org/typescript/7.0.2): Node `>=16.20.0` | Major-version compatibility with Next/build/lint/generated clients remains unproven. Do not approve simply because Node engine range matches. If proof fails, select a supported alternative in a reviewed packet amendment. |
| Tailwind CSS | `4.3.3` | [Registry metadata](https://registry.npmjs.org/tailwindcss/4.3.3) version observed | Exact lockfile entry plus compatible integration packages; template/browser/shadcn compatibility proof belongs to ARC-006/FND-002 |
| shadcn/ui and supporting UI tools | Not yet pinned | Source selects approach, not a validated template/CLI/component version | ARC-006 audits license/components and records CLI/source revision; do not execute an unreviewed installer |
| Docker / Compose | Exact project baseline not yet selected | Host Docker CLI observed; Compose and daemon untested | FND-003/ARC-007 verify host prerequisites, image architectures/digests and resource limits |

Research also queried Go module proxy `@latest` endpoints and npm registry `/latest` endpoints to discover candidates; versioned links above should be retained with eventual lockfiles and checksums. Metadata is not a vulnerability scan, signature audit, dependency resolution or runtime test.

NATS, Redis, storage/scanner, observability and test-tool versions need owner-reviewed pins when their slices are selected. Python/FastAPI/Pydantic/pgvector are deferred to approved AI/search work; no `ai/` application is needed for the manual increment. Provider selections and container-service policies remain ARC-007/008 decisions.

### Selection rejection rules

Reject a combination if any runtime engine or Go minimum is unsatisfied, dependency resolution fails, unsupported/RC software is selected without approval, generated code cannot compile, peer dependencies require bypass flags, or required security checks fail. Do not use `--force`/legacy peer-dependency bypasses, unpinned image tags or lower assertions to conceal incompatibility.

## 4. Repository and generated-file conventions

After BID-ADR-005 approval, resolve `<frontend-root>` to `web/` in authorized design references before scaffolding. Retain the [repository architecture](../technical_design/02_REPOSITORY_ARCHITECTURE.md); do not create its entire future tree in one task.

- `backend/cmd/`: composition for API/worker/migrator as actually required.
- Owner-module `domain/`: no Fiber, SQL driver, provider SDK or other module adapter imports.
- `application/`: use cases and ports; authorize transitions and own transaction orchestration.
- `adapters/`: HTTP/SQL/provider implementations; no direct access to another module's private tables/repositories.
- `web/src/features/`: domain UI orchestration; `components/ui/`: primitives without procurement policy.
- Go HTTP APIs remain the authority; no direct PostgreSQL or privileged worker/AI access from Next.js.
- Reviewed OpenAPI/events are authoritative contract inputs. Generated clients and sqlc output are committed with generator versions and reproducible regeneration checks; never hand-edit generated output.
- Commit migration source, Go module sums and the single npm lockfile. Released migrations are immutable; changes use new compatible migrations.
- Do not commit secrets, local volumes, dependency/build output, private test data or signed URLs. Do not delete unrelated pre-existing files as cleanup.
- Adding a dependency requires purpose, owner, license/maintenance/security review and actual compatibility evidence.

## 5. Migration proof required before acceptance

Goose is a runner, not a substitute for Bidora's tenant-fleet orchestration. DB-001 must implement and prove any missing checksum, compatibility, canary and checkpoint controls explicitly rather than attributing them to the tool.

1. Apply reviewed control and tenant migration sets to empty isolated databases with separate migrator/runtime roles.
2. Upgrade from a supported previous schema and validate immutable migration history/checksums.
3. Demonstrate runtime DDL denial and migration locking under concurrent attempts.
4. Inject failure, pause a bounded tenant batch and resume from its durable checkpoint without skipping or duplicating schema changes.
5. Exercise binary/schema compatibility and reviewed forward-fix; never destructive automatic down-migration.
6. Generate pgx/v5 queries using pinned sqlc, compile and verify transaction rollback of state/audit/outbox with real PostgreSQL.

No proof was executed here. DB-001 is not Ready solely because this packet names Goose.

## 6. Proposed branch, review and command conventions

Once explicitly authorized to initialize Git and choose hosting:

- Protected `main`, no direct pushes for application changes; one bounded ticket per review where feasible.
- Branch pattern `bid/<ticket-id>-<short-description>`; small meaningful commits, ticket ID in review description.
- Required engineering review; domain/security/operations reviewers for affected policy and sensitive boundaries. No named reviewer assigned by inference.
- CI must enforce actual repository check targets and retain failures/skips; nobody may label nonexistent checks passed.
- Local scripts and CI invoke the same version-pinned commands. Final command names are created and verified by foundation/CI tickets, not declared runnable now.
- Promotion uses the reviewed artifact, not a new untested build. Deployment approval remains a separate gate.

## 7. AGENTS review findings

Reviewed [root AGENTS](../../AGENTS.md), the repository design and engineering/environment standards.

| Finding | Disposition |
|---|---|
| Documentation-only authorization and no duplicate frontend roots | Still correct; keep enforced until explicit implementation/root approval |
| Product sources under `docs/product/`; no `docs/specifications/` target | References match actual structure; no path correction required |
| Domain/port boundaries, real API UI, no production mocks, immutable evidence | Preserve unchanged; enforce in future tests/import checks |
| AGENTS section J requires current decision register before work | BID-ADR-006 approval is now visible there; scope approval does not remove remaining policies |
| Tool pins and execution commands do not exist yet | Add only under authorized foundation tickets; do not present proposed commands as available |
| Path variable resolved by approved ADR-005 | Root AGENTS and repository design now resolve all `<frontend-root>` placeholders to `web/`; no second app root permitted |

Root AGENTS was updated only to record accepted path/tool decisions. Its authorization, safety and business-policy constraints remain unchanged; framework-generated instructions must not replace them.

## 8. Remaining gates and recommended order

1. **Complete:** user approved `web/` + `backend/`, Goose and npm; BID-ADR-005/008 are synchronized.
2. **Node metadata gap resolved:** official release/schedule requests succeeded on retry; updated candidates above. Review dependency/security compatibility and assign engineering reviewer; complete or defer unneeded pins explicitly.
3. Approve the bounded version-selection packet, with runtime proof attached to the corresponding foundation tasks; do not label this metadata review a passing build.
4. Resolve ARC-007's local configuration/security/resource baseline needed by FND-003. Resolve ARC-006's frontend primitive/page approach before FND-002.
5. Under explicit implementation authorization, implement API-001 and FND-003, then FND-001 and FND-002 with their full prerequisites. Use the [execution runbook](08_EXECUTION_RUNBOOK.md).

ARC-001's procurement/domain walkthrough remains separately open. None of these steps approves identity, business review/waiver policy, production provider or deployment.

## 9. Evidence and ticket disposition

- Read-only workspace and host CLI observations recorded above; `git status` correctly failed because no repository exists.
- Official Go/PostgreSQL/Next documentation and upstream package metadata fetched. Initial Node DNS failure was resolved on retry; Node release/schedule and npm 11.19.1 metadata now recorded.
- Declared minimum/peer-range comparison performed; **no experimental compatibility/security verification**.
- No framework/install/build/container/migration commands executed; application roots remain unchanged.
- Ticket remains **Proposed** pending exact version/convention review; approved path/tool choices are no longer blockers. User identity is the requesting conversation user; no formal organizational title or independent reviewer sign-off is inferred.

Next work: prepare ARC-007's local environment and QA baseline while candidate version/security review remains open. Candidate version acceptance is separate from passing runtime tests. The local installation inventory is not authority to bypass those tests.
