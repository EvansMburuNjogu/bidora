# Security implementation workstream

## 1. Purpose
Make security an acceptance condition of each slice, not a final patch phase.

## 2. Scope
Identity, tenant/data isolation, files, browser/API, secrets, audit, dependency supply chain and future sealed/AI boundaries.

## 3. Product specifications
[Security specification](../product/14_SECURITY_PRIVACY_COMPLIANCE.md), SEC-001–009, BR-006/014/043/051.

## 4. Technical designs
[Security matrix](../technical_design/19_SECURITY_ARCHITECTURE.md), [identity](../technical_design/11_AUTHENTICATION_AUTHORIZATION.md), [storage](../technical_design/15_DOCUMENT_STORAGE_DESIGN.md).

## 5. Prerequisites
Named security/privacy owner, threat model for scoped operation, data classification and explicit authority matrix.

## 6. Required decisions
BID-ADR-007/010/011/013/014/017; unresolved crypto, retention, approval or provider policy blocks related exposure.

## 7. Dependencies
Security tests accompany first implementation of every boundary. Phase 8 is cumulative assessment, not permission to defer controls.

## 8. Target files
Future middleware/policy tests, DB roles, storage/network policies, secret config, scanner CI and runbooks.

## 9. Sequence
Trace assets/actors/trust boundaries; define abuse cases; implement least privilege; add negative/race tests; scan dependencies/secrets; exercise revocation/containment; independent review before pilot and sealed release.

## 10. Domain/security
No implicit support access, no private data in logs or browser caches, no unaudited override. Parameterized SQL, CSRF, output sanitization, SSRF-safe fetch, immutable scan target and restricted financial DTOs are mandatory.

## 11. Database/API/frontend
App DB roles lack DDL/audit mutation; APIs conceal unauthorized resources; frontend action hints cannot grant access. Sensitive package classification follows contents.

## 12. Tests
Cross-tenant IDs/counts/objects/jobs, role escalation, recovery replay, concurrent last-admin removal, approval/edit race, upload bombs, SSRF, XSS/export formulas, revoked signed-access limits and secret scanning.

## 13. Acceptance
Zero cross-tenant/sealed confidentiality defects; critical/high findings remediated or handled through permitted formal process; no unsupported compliance claim.

## 14. Done
Threat model, reviewed controls, reproducible attack tests, incident owners and residual risk record.

## 15. Risks
Security checklist without execution; third-party scanner green status cannot prove tenant policy. Require behavior-based evidence.

## 16. Evidence
Finding IDs/severity/scope, actual scan versions/results, test reports, remediation verification and reviewer sign-off.

## 17. Exclusions
Exploitability claims without evidence, disabling controls for tests, custom sealed crypto before independent design review.
