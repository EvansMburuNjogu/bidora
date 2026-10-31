# BID-SEC-001 — Threat-model tenant, identity, document and evidence boundaries

## Metadata
- Ticket ID: BID-SEC-001
- Epic: E-SEC
- Type: Security
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-005, BID-ARC-007, BID-ARC-008
- Blockers: Approved identity/privacy/upload policies
- Milestone: M0

## 1. Objective
Approve concrete abuse cases and control/test ownership before private-data implementations.
## 2. Business Context
Bidora handles confidential pricing, identity records and procurement evidence with high insider and isolation risk.
## 3. Scope
Assets/trust boundaries, auth/session/CSRF, router/cache/jobs, uploads/previews, pricing exports, secrets/supply chain and external fetch; future sealing separately gated.
## 4. Out of Scope
Claiming implemented controls, pentesting nonexistent services, substituting generic OWASP list for domain cases.
## 5. Technical Requirements
Map each threat to owner, enforcement point, negative fixture and release gate. Include stale context, last-admin race, revoked queued actor, scan/serve TOCTOU and retained evidence deletion.
## 6. Proposed Implementation Approach
Walk approved sequence diagrams with security/data owners; document attacker capabilities and mitigations; specify test oracles and incident containment; link unresolved risks to decisions.
## 7. Dependencies
[Identity](../architecture/BID-ARC-005.md), [privacy/operations](../architecture/BID-ARC-007.md), [files/providers](../architecture/BID-ARC-008.md).
## 8. Acceptance Criteria
- Given a private-data path, then authorization/isolation and telemetry-redaction tests are assigned.
- Given missing policy, then implementation exposure remains blocked.
- Given platform support actor, then default private access is explicitly denied.
## 9. Testing Requirements
Threat-model tabletop, data-flow review and negative-case traceability checks.
## 10. Definition of Done
Security-approved threat/control/test matrix and residual risk ownership, not runtime security certification.
## 11. References
[Security §4](../../product/14_SECURITY_PRIVACY_COMPLIANCE.md), [security design](../../technical_design/19_SECURITY_ARCHITECTURE.md), [security plan](../../implementation/15_SECURITY_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Independent assessment is still required before customer pilot.
