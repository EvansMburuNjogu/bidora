# Identity and authorization implementation

## 1. Purpose
Establish verified identity and explicit tenant authority before administrative/business functionality.

## 2. Scope
Login/verification/recovery/MFA, opaque sessions, memberships, role scope, context and step-up policy.

## 3. Product specifications
SR-IAM-001–008, PR-SH-001/002, RULE-IAM-001–003; [security §§5–7](../product/14_SECURITY_PRIVACY_COMPLIANCE.md).

## 4. Technical designs
[Identity](../technical_design/11_AUTHENTICATION_AUTHORIZATION.md), [tenancy](../technical_design/04_MULTI_TENANCY_DESIGN.md), [API](../technical_design/10_API_CONTRACT_DESIGN.md).

## 5. Prerequisites
Identity threat model, reviewed contracts, control DB migrations and safe local mail/testing provider.

## 6. Required decisions
BID-ADR-007 now selects Go-owned email/password; Argon2 calibration, normalization/verification, recovery/MFA/lifetimes remain pending. BID-ADR-011 governs role/approval policy; BID-ADR-016 governs cookie/CSRF.

## 7. Dependencies
Foundation/control schema first; identity and minimal audit before exposed provisioning; tenant membership follows registry.

## 8. Target files
Future platform/identity, shared/authz, control queries, authentication routes/components and security test fixtures.

## 9. Sequence
Approve policy matrix; implement token hashing/challenge consumption; rotation/revocation; MFA/recovery; generic errors/rate limits; `/me`; membership grants/context; last-admin lock; job-time reauthorization; invitation/session-management contracts before UI.

## 10. Domain/security
No standing platform private-data access. Hash opaque session tokens; encrypt factor secrets; one-use recovery codes. CSRF on cookie writes; no browser localStorage tokens. Session epoch and membership status remain authoritative.

## 11. Database/API/frontend
Control DB identity/membership authority; tenant profile is assignment projection only. Bootstrap drives navigation; API independently checks capability/resource/assurance. Context generation prevents misrouted stale-tab writes.

## 12. Tests
Fixation, token/factor/recovery replay, enumeration, CSRF, wrong origin, role escalation, last-admin race, revoked job/download and multi-tab context.

## 13. Acceptance
Verified user completes login/logout/recovery and unauthorized operations fail consistently without metadata leak; privileged MFA enforced.

## 14. Done
Threat model, policy matrix, browser/integration negative tests and security review accepted.

## 15. Risks
Self-managed auth workload; managed provider outage; stale allow-cache. Choose provider deliberately and define failure-closed behavior.

## 16. Evidence
Synthetic identity E2E, cookie/rotation tests, revocation trace, role matrix and unresolved assurance policies.

## 17. Exclusions
Unapproved SSO/SAML/SCIM, default admin bypass, guessed approval roles, credentials in fixtures/logs.
