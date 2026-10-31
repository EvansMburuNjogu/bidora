# BID-ARC-005 — Select identity architecture and approve authorization policies

## Metadata
- Ticket ID: BID-ARC-005
- Epic: E-ARC
- Type: Security
- Priority: P0
- Status: Proposed
- Complexity: M
- Dependencies: None
- Blockers: BID-ADR-007, BID-ADR-016; security-owner approval
- Milestone: M0

## 1. Objective
Choose identity delivery and define session/context assurance before auth persistence and APIs.
## 2. Business Context
Bid pricing, approvals and tenant data depend on trustworthy human identity and revocable authority.
## 3. Scope
Go-owned password versus OIDC decision; MFA/recovery, cookie/CSRF, session rotation/expiry, membership freshness, last-admin policy, platform support boundaries and service identities.
## 4. Out of Scope
Provider procurement, guessed lifetimes, implementing both identity alternatives.
## 5. Technical Requirements
Separate business authorization from provider authentication; define grantable capabilities/scopes and deny precedence. Resolve context generation and signed-URL revocation windows. Include factor enrollment/recovery and session-list/revoke contract requirements missing from current cards.
## 6. Proposed Implementation Approach
Threat-model account and tenant boundaries; compare providers/operational cost; define assurance and abuse budgets; review fail-closed outage behavior; sign policy matrix.
## 7. Dependencies
None. Product approval is required for role/assurance consequences; security approval for credential/session design.
## 8. Acceptance Criteria
- Given a revoked membership, then an explicit freshness guarantee applies to API, job execution and download.
- Given platform administrator, then private content requires a separate approved support grant.
- Given MFA recovery, then policy prevents reusable recovery challenges.
## 9. Testing Requirements
Review fixation, CSRF, recovery replay, enumeration, context race and last-admin concurrency scenarios.
## 10. Definition of Done
Accepted identity/security decision with owners, threat cases, configuration budgets and interface implications.
## 11. References
[Security §§5–7](../../product/14_SECURITY_PRIVACY_COMPLIANCE.md), [identity design](../../technical_design/11_AUTHENTICATION_AUTHORIZATION.md), [identity plan](../../implementation/08_AUTHENTICATION_AUTHORIZATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
No provider or custom password system is approved merely because it is recommended in prose.
