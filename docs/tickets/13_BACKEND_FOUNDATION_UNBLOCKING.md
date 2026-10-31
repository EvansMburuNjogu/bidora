# Backend foundation: blocker review and next approval

Recorded: 2026-10-17. Status: **Local-only plan and resource budget APPROVED; execution BLOCKED on runtime and reviewed technical prerequisites**.

## Current execution mode and result

The user subsequently chose manual ticket execution and excluded subagents. No delegation will resume without reauthorization. [FND-005](foundation/BID-FND-005.md) is now **In Review / VERIFIED_TECHNICALLY** after direct implementation of the isolated resource validator, external tests and backend-local usage notes. Full backend race tests, vet and build passed; the new package has 100% statement coverage. No runtime startup wiring, database integration or web change was made. FND-004 retains its independent Docker/pins/security/role gates and awaits accepted child evidence. The launch records below are historical failed attempts, not active agents.

## Docker-free split and launch authorization

The user subsequently instructed: **“please split and start the sub agents, i want them running”**. [FND-005](foundation/BID-FND-005.md) is now Ready for disconnected, standard-library local resource validation, centralized unit tests and backend-local usage notes. It uses the approved local ceilings without new services, tool pins, business policies or HTTP changes. The requesting user is the scope/result reviewer; independent security acceptance is not inferred. FND-004 remains Blocked and now explicitly depends on reviewed FND-005 completion plus its existing Docker/pin/security/configuration-role gates. No original dependency is removed.

Launch is authorized by this instruction under the unchanged persistent AI ceilings. Workers run sequentially, not concurrently: shared backend edits and budget reservations must not race. Use an already-authenticated, supported priced model; do not weaken the cap adapter to accommodate the foreground Codex model. Outcomes and usage are recorded in `.pi/backend-runs/`, never inferred from a successful process spawn.

### Actual launch result

At 2026-10-17T21:00:31Z the controller launched a real isolated Pi process for FND-005 using `anthropic/claude-haiku-4-5` with thinking off. It stopped at 21:00:33Z with `WORKER_GUARD_STOP`: Anthropic OAuth refresh failed with `invalid_grant` / `Refresh token expired`, before the first model request. A prior non-refreshing `pi auth check` reported cached credentials ready; that was not proof they could refresh.

Report: `.pi/backend-runs/1792270831599-99908/report.json`; private event/error logs are adjacent. The worker made no tool calls or backend edits. Persistent request, token and cost counters remain zero, and the controller lock was released. **No worker remains running.** Do not repeatedly retry the expired credential or weaken the unsupported Codex budget adapter.

Operator recovery in Pi: `/login anthropic`, select `anthropic/claude-haiku-4-5` with `/model`, then `/backend-budget` and `/backend-run`. FND-005 remains Ready on its technical prerequisites; provider authentication is the runtime launch blocker. Docker is not required for this packet.

### Requested GPT-6 Astra launch

The user subsequently requested ChatGPT GPT-6 Astra. A controller invocation explicitly selected `openai-codex/gpt-6-astra` with low thinking for FND-005. The child stopped before sending a model request: `No audited output-cap adapter for openai-codex-responses; stop rather than run uncapped`. Report: `.pi/backend-runs/1792271015449-752/report.json`. All persistent usage counters remain zero and no worker is running. The installed Codex adapter's request builder does not set an output-token cap; changing the selected model alone does not make the existing 4,096-output-token control enforceable. No budget limit was removed or increased. Supporting this provider requires verified adapter behavior or explicit approval of revised controls, not Docker or another Anthropic login.

## Approval record and follow-up verification

The requesting user explicitly approved: **“Approve the local-only backend foundation plan, with 2 CPUs, 2 GiB RAM and a 10 GiB storage budget.”** Recorded 2026-10-17. This accepts the bounded plan below, including candidate verification and preparation of local child/dependency records. It does not approve production, identity, procurement rules, unchecked version pins or invented independent sign-off. Do not request the same local-scope/resource approval again.

Prepared [BID-FND-004](foundation/BID-FND-004.md), a Blocked local-only child of FND-003. It has no hard dependency on completion of the broader ARC-002/007 parents; its specific local review gates remain explicit blockers. The subsequent split above adds only FND-005 as its local implementation prerequisite. Original parent and downstream dependency edges are unchanged. The child is included in the worker manifest, but not dispatchable until Ready.

Follow-up read-only checks after approval:

| Check | Actual result / limitation |
|---|---|
| Docker server query | Still fails: configured local daemon is unreachable. No engine was started or installed. |
| Host capacity (`sysctl -n hw.ncpu hw.memsize`; `df -h .`) | 8 logical CPUs, 17,179,869,184 bytes RAM (16 GiB), 271 GiB filesystem availability reported. This is not Docker VM allocation or a disk quota. |
| pgx v5.11.0 versioned Go proxy module metadata | HTTP 200; Go minimum 1.25.0 |
| Goose v3.29.0 versioned Go proxy module metadata | HTTP 200; Go minimum 1.26.0 |
| sqlc v1.31.1 versioned Go proxy module metadata | HTTP 200; Go minimum 1.26.0, toolchain go1.26.2 |
| Official PostgreSQL version policy | HTTP 200; 18.6 is present. No database runtime test performed. |
| Docker Hub `library/postgres:18.6` OCI index | Index bytes verified against registry digest `sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336` |
| Linux/amd64 image manifest | Selected unique declared host-compatible platform; manifest bytes match `sha256:885953109528ad3dfc90362b1a6f50a78620b5315be19f187753d267e484dc5b`. Compressed layer total: 162,486,340 bytes; no image layers pulled. |

Module sources are the versioned URLs in [packet 10](10_FOUNDATION_DECISION_PACKET.md). Image metadata came from `registry-1.docker.io/v2/library/postgres/manifests/18.6` and the selected manifest digest, using an ephemeral anonymous pull token that was not recorded. Metadata was retrieved with bounded HTTPS requests. The Go 1.27.2 health toolchain exceeds the declared Go minimums; actual resolution/build compatibility, license/security review, image scan/signature provenance and runtime behavior remain unverified for this new stack. These checks do not automatically accept the candidates.

The sections below preserve the original review/request context; the scope/resource approval above supersedes their pending-approval wording. The remaining immediate operator action is to start the installed Docker runtime and confirm the approved allocation.

## Authorization and boundaries

The user's instruction to “proceed” follows the backend-status report and authorizes progressing the blocker review. It is not interpreted as evidence that independent reviews occurred or that unspecified business, privacy, provider or deployment choices were approved.

This review preserves `web/`, product sources, ticket statuses and the worker's existing budget. No backend subagent/model request, database provisioning, image pull, migration, Git initialization or service startup was performed. The foreground conversation is not counted by the delegated-worker usage ledger.

## Findings: resolved decisions versus outstanding work

| Gate | Already established | What actually remains |
|---|---|---|
| ARC-001 / ADR-006 | User approved Option A; canonical register and release matrix reflect it | Procurement/domain walkthrough and relevant security/privacy/operations dispositions. This technical review cannot substitute for them. |
| ARC-002 / ADR-005/008 | `backend/`, sole frontend `web/`, Goose, npm; existing local Go health slice has actual test evidence | Exact broader tool/service baseline and engineering review. pgx/Goose/sqlc/PostgreSQL candidate metadata is not integration or migration proof. |
| ARC-007 / FND-003 | Local-only/synthetic/loopback/no-destructive-startup rules already proposed; health-only configuration exists | Explicit local-baseline approval, service pins/digests, host resource allocation and clean-start proof. Full production/privacy/recovery baseline remains separately unresolved. |
| Local runtime | Docker CLI and Compose available | Docker daemon unreachable at the configured local socket. This is a concrete operational blocker, not a missing package-manager choice. |
| Worker eligibility | Backend-only controller and budget controls configured | 46 manifest targets; none Ready. Architectural packets require documentation edits that the backend-only worker intentionally cannot make. Owner/controller review must resolve those prerequisites outside the child. |

Do not re-request approval for Option A, repository roots, Goose or npm. Conversely, do not mark their umbrella tickets Done simply because those selections or the health subset are complete.

## Fresh verification

Executed from repository root, with Go commands inside `backend/`:

| Command | Observed result |
|---|---|
| `date -u '+%Y-%m-%dT%H:%M:%SZ'` | `2026-10-17T20:51:00Z` |
| `go version` | `go1.27.2 darwin/amd64` |
| `go test -count=1 -race ./...` | Pass: centralized config and HTTP suites; production packages report no colocated tests as intended |
| `go vet ./...` | Pass, no diagnostics |
| `docker --version` | Docker `29.6.2`, build `dfc4efb1e2` |
| `docker compose version` | Docker Compose `5.3.1` |
| `docker info --format '{{.ServerVersion}}'` | **Fail:** cannot connect to the configured Docker socket; no server version obtained |

The combined shell command exited 1 because the final Docker query failed. This does not negate the separately observed passing Go checks. No claim of PostgreSQL, migration, tenant isolation, security scan, container architecture compatibility or recovery verification is made. Earlier security evidence and its unused-module advisory remain in [the health report](12_HEALTH_SLICE_EVIDENCE.md); no new security waiver is granted.

## Recommended next bounded authorization

**Recommendation: approve a local-only backend foundation increment, not the full ARC-002/007 tickets or production baseline.**

Requested scope:

1. Preserve the working Go 1.27.2 / Fiber 3.6.1 health profile. No web changes or frontend dependency upgrades.
2. Prepare reviewed backend-local child tickets under ARC-002/007/FND-003 for the work below; synchronize their master rows, dependency graph and worker target manifest before dispatch. Parent tickets and unrelated prerequisites stay open. This explicit change is necessary: merely adding a Ready ticket outside the manifest will not schedule it.
3. Verify the previously researched PostgreSQL 18.6, pgx 5.11.0, Goose 3.29.0 and sqlc 1.31.1 candidates using versioned upstream metadata, compatibility/license/security evidence and actual host-platform support. Record a PostgreSQL image digest before using it. **These are candidates, not newly accepted pins.** Stop and present alternatives if a candidate fails; never silently use `latest`.
4. Implement only the approved local PostgreSQL profile/configuration and its synthetic integration harness under `backend/`. Use separate control and at least two tenant databases for the later isolation proof, distinct provisioner/migrator/runtime roles, loopback-only exposure, ignored local secrets and explicit configuration validation. Any bootstrap SQL belongs to this reviewed local task; business schemas and public provisioning endpoints are excluded.
5. Preserve volumes on ordinary startup. No automatic migration/seed, destructive reset, production credentials, public tunnel or remote shared database. Defer NATS/Redis/mail/storage/scanner until their scoped dependencies and decisions are approved.
6. Keep all Go tests under `backend/tests/`. Prove missing/unavailable dependency failure, wrong-environment rejection, repeat-start data preservation, runtime DDL denial and independent tenant connectivity before review. Never label connectivity alone as full tenant authorization/isolation acceptance.
7. Retain review-required stopping and the persistent US$5 modeled reservation / 20-request / 2-million-token budget. No automatic increases or resets. Provider billing ceilings remain a separate provider-side setting.

### Host prerequisites requiring an operator

- Start your installed Docker runtime, or choose an explicitly approved alternative; this review did not launch/install one.
- Confirm `docker info --format '{{.ServerVersion}}'` succeeds. CLI/Compose availability alone is insufficient.
- Confirm a **proposed local cap of 2 vCPU, 2 GiB RAM and a 10 GiB storage budget** is acceptable for this PostgreSQL-only experiment. These are resource limits to review, not measured production capacity; Compose memory/CPU settings do not automatically enforce a disk quota. Verify available disk separately and stop if capacity is insufficient.
- Do not supply production secrets or customer data. Generated local credentials must not be committed or echoed into reports.

### Approval wording

A precise response would be:

> Approve the local-only backend foundation plan in packet 13, including preparing the reviewed child-ticket/dependency changes and candidate verification, with the proposed local resource budget. Keep production, identity and procurement decisions blocked.

This approves preparing/verifying that scope, **not** fabricated engineering sign-off or automatic acceptance of failed version/security checks. Actual candidate results must be attached and reviewed before a child is marked Ready for installation/implementation. The user may name themselves as local engineering reviewer if appropriate; no professional role or independent security approval is inferred.

## After approval: execution sequence

1. Owner starts the local container runtime and confirms resource allocation.
2. Prepare the explicit child packets and validate candidates without changing protected sources or web code.
3. Review exact pins, configuration/role contracts, acceptance checks and the limited prerequisite amendment; record the actual approver and evidence. Full ARC-001/002/007 closure is not claimed.
4. Synchronize the reviewed child in ticket/master/dependency/scope records. Mark it Ready only when its real prerequisites are satisfied; validate graph and worker preflight again.
5. In Pi: `/backend-budget`, then `/backend-run` with scope/spend confirmation. Unknown pricing/unsupported model adapter must stop, not bypass budgeting.
6. Review the child result before accepting it; continue only through approved backend packets. Stop before identity/business behavior until the respective decisions close. Never continue to web work.

## References

- [Release approval and remaining walkthroughs](09_RELEASE_SCOPE_REVIEW.md)
- [Toolchain candidate packet](10_FOUNDATION_DECISION_PACKET.md)
- [ARC-007 local proposal and child-ticket rule](architecture/BID-ARC-007.md)
- [FND-003 requirements](foundation/BID-FND-003.md)
- [Canonical decisions](../technical_design/27_ARCHITECTURE_DECISIONS.md)
- [Worker controls and limitations](../../.pi/README.md)
