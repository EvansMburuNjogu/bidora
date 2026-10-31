# BID-OPS-002 — Deploy an immutable external-MVP candidate to isolated staging

## Metadata
- Ticket ID: BID-OPS-002
- Epic: E-OPS
- Type: DevOps
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-OPS-001, BID-FE-008, BID-OBS-002, BID-SEC-002
- Blockers: Approved host/region/network/secrets and candidate artifacts
- Milestone: M4

## 1. Objective
Provide production-like staging for end-to-end, security and recovery acceptance.
## 2. Business Context
Bid workflows must survive real private networking, role grants and storage/version policies—not only developer mocks.
## 3. Scope
Selected infrastructure/IaC, TLS edge, same-origin API, private DB/broker/storage, least-privileged identities, compatible migrations, digest promotion, feature flags and rollback rehearsal.
## 4. Out of Scope
Customer production activation, unapproved Kubernetes/cloud, real personal/customer data.
## 5. Technical Requirements
Separate provisioner/migrator/runtime; no public internal endpoints; schema compatibility and canary batching; health semantics; graceful drain; rollback only compatible binaries; safe secret rotation.
## 6. Proposed Implementation Approach
Provision approved target; inject secret references; run canary migration; deploy tested digests; smoke critical routes; record rollback/forward-fix and isolated test access.
## 7. Dependencies
[CI](BID-OPS-001.md), [complete UI](../frontend/BID-FE-008.md), [monitoring](../observability/BID-OBS-002.md), [isolation](../security/BID-SEC-002.md).
## 8. Acceptance Criteria
- Given failed canary, then rollout pauses without migrating all tenants.
- Given incompatible old binary, then rollback is rejected with forward-fix path.
- Given network probe, then internal services are inaccessible publicly.
## 9. Testing Requirements
Deployment smoke, network deny, secret rotation, migration pause and graceful restart evidence.
## 10. Definition of Done
Reproducible staging candidate, artifact/schema record and rehearsal runbook; release tests follow.
## 11. References
[Operations §§6/15](../../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), [deployment](../../technical_design/21_DEPLOYMENT_ARCHITECTURE.md), [DevOps plan](../../implementation/17_DEVOPS_DEPLOYMENT.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Staging deployment is not production-readiness approval.
