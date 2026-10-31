# BID-EVT-002 — Coordinate authorized durable jobs, cancellation and operator replay

## Metadata
- Ticket ID: BID-EVT-002
- Epic: E-EVT
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-EVT-001, BID-AUTH-002, BID-API-004
- Blockers: Accepted job/retry policy
- Milestone: M2

## 1. Objective
Expose truthful long-running operation state and recover jobs under current authority.
## 2. Business Context
A queued package or onboarding action is not complete simply because the HTTP request returned 202.
## 3. Scope
Worker leases/checkpoints, job status/cancel/retry contracts, bounded backoff, service/requester authorization, safe error codes and audited operator replay.
## 4. Out of Scope
Cancelling already irreversible effects, job payload as authority, fabricated progress.
## 5. Technical Requirements
Revalidate requester/service scope at execution and result query; lease ownership prevents concurrent completion; retries preserve operation identity; graceful shutdown leaves recoverable state; result IDs concealed from foreign tenants.
## 6. Proposed Implementation Approach
Implement job state machine and handlers; distinguish retryable/final errors; add control and tenant job stores; define external-effect checkpoint interface; expose polling API.
## 7. Dependencies
[Delivery](BID-EVT-001.md), [policy](../authentication/BID-AUTH-002.md), [job contract](../backend_contracts/BID-API-004.md).
## 8. Acceptance Criteria
- Given revoked requester, then private processing/result access fails safely.
- Given expired lease, then resume does not double-complete operation.
- Given replay, then actor/reason/scope are audited.
## 9. Testing Requirements
Lease races, restart, cancellation boundary, unauthorized job IDs and safe status API contract tests.
## 10. Definition of Done
Durable job API/worker with negative and recovery evidence; no false success states.
## 11. References
[LLD worker lifecycle](../../product/09_LLD.md), [events design](../../technical_design/17_EVENT_MESSAGING_DESIGN.md), [worker plan](../../implementation/12_EVENTS_WORKERS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Job idempotency does not reverse externally completed side effects.
