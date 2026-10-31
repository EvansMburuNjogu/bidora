# Bidora backlog execution runbook

## 1. Purpose and safety boundary

Use this runbook to take one approved ticket from selection through evidenced completion, or to resume interrupted work safely. Audience: engineering leads, developers, reviewers and coding agents.

This is an **execution procedure for the backlog**, not a production incident runbook. Only the local health slice is implemented; its actual start/check commands are in [README](../../README.md). No production deployment commands are established. Do not invent `make`, migration, test or deployment commands that do not exist.

The user subsequently authorized the bounded local health slice and its dependencies; see [execution evidence](12_HEALTH_SLICE_EVIDENCE.md). This does not authorize unrelated scaffolding, Git initialization, business migrations or deployment. Follow [AGENTS](../../AGENTS.md) and obtain explicit authorization for each implementation scope.

## 2. Required inputs

Before selecting work, read:

- [Master backlog](02_MASTER_BACKLOG.md) and the full individual ticket.
- [Dependency graph](03_DEPENDENCY_GRAPH.md) and [vertical slices](04_DELIVERY_ROADMAP.md).
- [Ticket standards / Ready and Done](06_TICKET_STANDARDS.md).
- Applicable product, design and implementation references in the ticket.
- [Canonical architecture decisions](../technical_design/27_ARCHITECTURE_DECISIONS.md) and [known design gaps](../technical_design/QUALITY_REPORT.md).

Identify an accountable implementer and reviewer; policy changes also need the appropriate product/security/privacy owner. Do not invent named assignments or infer approval from silence.

## 3. Select the next eligible task

1. Inspect the actual workspace and current ticket status. Documentation is not evidence of deployed behavior.
2. Select the earliest useful operation in the approved vertical slice, not simply the lowest ticket number or highest priority.
3. Read every hard prerequisite in the ticket metadata. Verify its required acceptance evidence, not only its status label.
4. Check non-ticket blockers: policy decisions, implementation authorization, missing contracts, external credentials/providers and required runtime children.
5. If any prerequisite is unresolved, retain **Blocked** and record the missing artifact, owner role and next action. Work only on independently authorized tasks.
6. If all Ready criteria hold, obtain review and update the ticket and master row together to **Ready**.

### Starting from the current documentation baseline

Begin with owner review of ARC-001/002/003/005/006/007/008. ARC-004 follows domain/identity decisions; SEC-001 follows its policy prerequisites. ARC-009 is future sealing discovery, not a requirement to build the manual external workflow.

After applicable approvals and explicit implementation authorization, the foundation sequence is API-001 plus FND-003 → FND-001 → FND-002, with the additional prerequisites recorded in each ticket. Do not create both `web/` and `frontend/`; resolve BID-ADR-005 first.

## 4. Prepare a bounded execution packet

Record the following in the authorized work record before editing:

```text
Ticket:
Business operation / actor:
Authorized files and work type:
Implementer / reviewer:
Accepted source requirements and ADR versions:
Prerequisite evidence:
Contracts and schema versions:
Planned changes:
Positive / negative / concurrency / recovery tests:
Commands available in this repository:
Rollout / exposure / recovery plan:
Remaining blockers:
```

Confirm that the task can be reviewed and recovered independently. Split oversized implementation work before starting; preserve the parent ID and link approved children. Future design packets must produce runtime/UI/test children before their capabilities can be implemented or tested as complete.

For a design-only task, replace implementation steps with reviewed artifact deliverables. Its Done means accepted design, not running software.

## 5. Implement the approved operation

Once authorized, move the ticket and master row to **In Progress** and follow this order:

1. **Contract:** approve request/response/event/page shapes, permissions, validation, safe errors, versions and idempotency semantics.
2. **Persistence/domain:** approve fields/constraints and state guards; implement compatible migrations and domain rules in the owning module.
3. **Server:** enforce current identity, tenant context, capability, state and classification. Commit state/audit/outbox atomically in the owning database.
4. **External work:** use durable checkpoints and reconciliation for storage, broker, mail and jobs; do not hold DB transactions during provider calls.
5. **Frontend:** consume the accepted typed contract through the real backend. Include loading, empty, invalid, forbidden, not-found, conflict and retry states.
6. **Verification:** run relevant tests as the change develops; extend the tenant-isolation regression for each exposed path.

Stop if implementation reveals an unresolved business rule. Do not choose an arbitrary waiver, self-approval rule, deadline interpretation or provider to keep coding.

### Non-negotiable checks

- No default tenant, unverified tenant selector or cross-tenant SQL.
- Private pricing absent from unauthorized responses, not merely hidden in UI.
- Clean exact-version evidence only; scanner uncertainty remains quarantined.
- Immutable candidates, decisions, packages and proof; corrections append history.
- Revalidate queued work and downloads against current authority.
- A 202 response means pending work, not successful submission or delivery.
- AI remains advisory; no automatic approval, submission, opening or award.

## 6. Verify and review

Discover commands from the actual repository scripts/tooling and approved task packet. Record exact commands, environment/tool versions, exit results and sanitized artifact locations. Never substitute plausible command names for executed checks.

| Change surface | Minimum relevant evidence |
|---|---|
| Contract | Schema lint/conformance, examples, generated-client drift and compatibility |
| Persistence | Clean and supported upgrade migrations, constraints, role denials, transaction rollback |
| Tenant/auth | Two-tenant negative cases, revoked/stale authority, context switching and no metadata leakage |
| Domain transition | Valid/invalid states, expected-version conflict, duplicate/lost-response retry, immutable audit |
| Worker/provider | Crash/duplicate/out-of-order delivery, ambiguous effect reconciliation, bounded retry and current authorization |
| Document/package | Quarantine/type/hash/access, exact version/fingerprint, storage-success/DB-failure recovery |
| Frontend | Real API journey, all failure states, keyboard/mobile, context race and restricted payload checks |
| Release-impacting change | Applicable security, accessibility, load, restore and UAT evidence |

A skipped or unavailable required test remains an unmet gate. Do not weaken assertions, isolation or constraints to make tests pass. Move to **In Review** only with an honest evidence report and known limitations.

The reviewer checks source/decision alignment, implementation boundaries, tests, recovery and documentation. Required fixes return to In Progress. New unresolved prerequisites move the task to Blocked.

## 7. Stop, recover and resume

| Trigger | Immediate action | Resume condition |
|---|---|---|
| Unapproved policy or conflicting source | Stop affected operation; capture exact requirement and decision alternatives | Accountable owner accepts a resolution and contracts/tests are updated |
| Tenant leakage or authorization bypass | Stop exposure; preserve sanitized evidence; notify security owner; do not waive | Fix reviewed, original attack regression passes and exposure is authorized |
| Uncertain external effect | Do not blindly retry; inspect durable operation/effect identifiers | Reconcile provider and owning DB state; use approved idempotent recovery |
| Failed migration or partial provisioning | Pause scoped rollout; retain checkpoint and diagnostics; do not perform destructive auto-down | Reviewed forward-fix/retry or compatible recovery succeeds |
| Test environment unavailable | Report unverified checks and retain gate | Required environment restored and checks actually executed |
| Interrupted agent/developer session | Stop new edits; leave a precise handoff | Successor reinspects files, approvals, operation state and prior evidence |

Recovery actions themselves require authorization. Never delete unrelated files, overwrite a live DB or erase historical procurement evidence to simplify recovery.

On resume, read the handoff, inspect actual changes, verify prerequisite/decision versions and rerun checks affected by subsequent edits. Previous test results apply only to their recorded artifact/version.

## 8. Close the ticket truthfully

1. Compare actual outcomes against every acceptance criterion and required test.
2. Verify no unresolved blocker is hidden by a successful happy path.
3. Obtain required reviewer/UAT disposition and update authorized docs/runbooks/contracts.
4. Record the completion evidence below.
5. Update individual ticket and master status together. Mark **Done** only when its type-specific Definition of Done is satisfied.
6. Recheck dependency IDs, master consistency, local links and graph acyclicity after backlog changes.
7. Evaluate downstream tickets individually; never bulk-mark them Ready merely because one dependency closed.

```text
Ticket / status:
Requirement and decision references:
Changed files and artifact/schema/contract versions:
Implemented or designed behavior:
Verification commands and actual pass/fail/skipped results:
Evidence locations and reviewer disposition:
Remaining risks / blockers / deferred work:
Migration / recovery / feature-exposure state:
Next eligible action:
```

Merged code, accepted design and production release are different events. Feature exposure still requires the applicable [milestone gates](05_RELEASE_MILESTONES.md). REL-001 accepts scoped MVP evidence, REL-002 governs pilot, REL-003 governs human-authorized production promotion; future capabilities have separate gates.

## 9. Handoff checklist

- [ ] Ticket and master agree on status/dependencies.
- [ ] Allowed scope and approvals are explicit.
- [ ] Current files and pending changes are described; no unseen work claimed.
- [ ] Exact checks and results distinguish executed, failed, skipped and unavailable.
- [ ] Operation/job/migration state and safe recovery next step are recorded where relevant.
- [ ] No secrets, raw bid content or private production data appear in handoff artifacts.
- [ ] Next owner can continue without guessing business policy or repeating an uncertain external effect.

## 10. Maintaining this runbook

When runnable tooling and environments are approved, add verified command references and links to dedicated operational runbooks through separately authorized changes. Production incident procedures should be owned by [OBS-002](observability/BID-OBS-002.md), deployment by [OPS-002](devops/BID-OPS-002.md), and restore/reconciliation by [OPS-003](devops/BID-OPS-003.md). This document does not claim those systems or procedures have been implemented or exercised.
