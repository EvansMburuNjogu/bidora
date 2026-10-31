# BID-TEN-001 — Provision isolated workspaces through resumable checkpoints

## Metadata
- Ticket ID: BID-TEN-001
- Epic: E-TEN
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-AUTH-002, BID-EVT-002, BID-DOC-001, BID-TEN-003
- Blockers: Accepted placement/secret/quota policy
- Milestone: M2

## 1. Objective
Create a tenant database, credentials, storage namespace and administrator without partial activation.
## 2. Business Context
New bidding teams need a usable private workspace with visible recovery when onboarding fails.
## 3. Scope
Authorized workspace request/status, registry reservation, checkpoint worker, role/secret allocation, common/type migration, seed, storage policy, admin membership and ACTIVE transition.
## 4. Out of Scope
Hybrid tenants, private support access, runtime API database-creation privilege.
## 5. Technical Requirements
Stable idempotency/resource identity; ownership verification before adopting existing resource; durable checkpoint/audit; no ACTIVE until all prerequisites pass; safe retry/cleanup and structured job errors.
## 6. Proposed Implementation Approach
Use privileged provisioner identity distinct from API; implement checkpoint leases and validation; inject failures after each external effect; expose approved progress contract.
## 7. Dependencies
[Policy](../authentication/BID-AUTH-002.md), [jobs](../events_workers/BID-EVT-002.md), [storage namespace](../documents/BID-DOC-001.md), [entitlements](BID-TEN-003.md); API-002 inherited.
## 8. Acceptance Criteria
- Given retry after database creation, then the same owned database resumes without duplicate membership.
- Given name collision with foreign resource, then provisioning fails safely.
- Given incomplete storage/migration, then tenant remains non-active.
## 9. Testing Requirements
Checkpoint fault injection, duplicate request, least-privilege and multi-tenant provisioning tests.
## 10. Definition of Done
Observable resumable onboarding and runbook with real isolated resources and evidence.
## 11. References
[SR-TEN](../../product/04_SRD.md), [tenancy](../../technical_design/04_MULTI_TENANCY_DESIGN.md), [provisioning plan](../../implementation/09_TENANT_PROVISIONING.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Resource cleanup is reviewed and ownership-bound, never blind deletion.
