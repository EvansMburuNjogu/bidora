# BID-BE-015 — Maintain approved Tenderpreneur profile and knowledge

## Metadata
- Ticket ID: BID-BE-015
- Epic: E-BE
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-AUTH-002, BID-API-012, BID-DOC-002, BID-DB-003
- Blockers: Final profile field/classification catalogue, knowledge reviewer/grant policy and branding asset/template limits
- Milestone: M7

## 1. Objective
Provide versioned, permission-scoped organization facts and evidence that humans can approve for reuse by contributors and AI.
## 2. Business Context
Powerful drafting cannot safely rely on invented company experience or unreviewed documents.
## 3. Scope
Organization details, setup readiness, versioned logo/letterhead/template draft-preview-activate, capabilities, certifications, staff, projects, references, standard responses, source-document links, draft/review/approve/revoke/version and expiry applicability.
## 4. Out of Scope
Public company directory, automatic approval from uploads, executable/remote branding assets and unrestricted pricing/personnel disclosure.
## 5. Technical Requirements
Tenant-private persistence; field classification; immutable approved and branding versions; clean bounded assets with no scripts/macros/remote fetch; exact clean source links; expected versions; approval evidence; permission-filtered retrieval/index eligibility; edits create new draft versions.
## 6. Proposed Implementation Approach
Define bounded aggregates by knowledge type; implement commands/reads through tenant owner module; expose approved retrieval port to AI rather than tables.
## 7. Dependencies
[Membership](../authentication/BID-AUTH-002.md), [contracts](../backend_contracts/BID-API-012.md), [documents](../documents/BID-DOC-002.md), [tenant persistence](../database/BID-DB-003.md).
## 8. Acceptance Criteria
- Given unapproved/expired knowledge, then AI cannot cite it as valid evidence.
- Given revoked authority, then search/retrieval immediately denies.
- Given approved-version edit, then history remains immutable and new content is draft.
- Given active letterhead change, then prior proposal/package branding remains bound to its old exact version.
- Given clean but unreviewed company document, then it may be searchable as restricted source but not presented as approved company evidence.
## 9. Testing Requirements
Classification, approval/edit races, expiry, cross-tenant retrieval, foreign document and AI-port authorization tests.
## 10. Definition of Done
Real tenant profile-to-approved-knowledge journey passes isolation and version evidence.
## 11. References
[PR-TP-001/002](../../product/03_PRD.md), [onboarding/context](../../../backend/docs/phase-1-tenderpreneur-onboarding-context.md), [branding export](BID-BE-017.md), [AI controls](../../technical_design/18_AI_SEARCH_ARCHITECTURE.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Personal and financial profile fields require data-minimization review before inclusion.
