# BID-BE-017 — Apply versioned organization branding to proposal exports

## Metadata
- Ticket ID: BID-BE-017
- Epic: E-BE
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-BE-004, BID-BE-015, BID-API-012, BID-DOC-002
- Blockers: Approved letterhead fields/assets, template rules and PDF/DOCX rendering/security review
- Milestone: M7

## 1. Objective
Apply one exact approved organization letterhead/template version to deterministic proposal PDF/DOCX exports without changing historical packages.
## 2. Business Context
Tenderpreneurs should configure branding once and safely reuse it across future proposal documents.
## 3. Scope
Letterhead/template selection, safe logo/background assets, synthetic preview, exact-version snapshot, deterministic PDF/DOCX rendering, hash/manifest provenance and supersession behavior.
## 4. Out of Scope
Executable templates/macros, remote fonts/resources, electronic signatures, arbitrary office-suite editing and rewriting past exports.
## 5. Technical Requirements
Only clean approved bounded assets; no scripts/macros/external fetch; immutable branding snapshot; deterministic bytes for the same inputs; current authorization at preview/export; historical package references remain resolvable.
## 6. Proposed Implementation Approach
Consume BE-015's owner port for an exact branding version; validate assets through DOC-002; extend the existing deterministic renderer and package manifest through reviewed contracts.
## 7. Dependencies
[Proposal/export](BID-BE-004.md), [organization knowledge/branding](BID-BE-015.md), [contracts](../backend_contracts/BID-API-012.md), [documents](../documents/BID-DOC-002.md).
## 8. Acceptance Criteria
- Given identical proposal and branding versions, then exported bytes/hashes reproduce.
- Given later branding change, then historical export/package remains unchanged.
- Given unsafe or unclean asset, then preview/export fails closed.
## 9. Testing Requirements
PDF/DOCX determinism, hostile asset/XML, Unicode, historical version, authorization, size and cross-tenant asset tests.
## 10. Definition of Done
Preview and export use exact approved branding with reproducible bytes and complete manifest/audit evidence.
## 11. References
[Onboarding/context](../../../backend/docs/phase-1-tenderpreneur-onboarding-context.md), [Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Branding is presentation, not proof of signature or external acceptance.
