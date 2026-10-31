# BID-ARC-008 — Approve document formats, provider boundaries and pilot quotas

## Metadata
- Ticket ID: BID-ARC-008
- Epic: E-ARC
- Type: Design
- Priority: P0
- Status: Proposed
- Complexity: M
- Dependencies: None
- Blockers: BID-ADR-014, BID-ADR-015, BID-ADR-017
- Milestone: M0

## 1. Objective
Resolve policies needed to safely accept tender files and generate reproducible bid packages.
## 2. Business Context
A package must contain the exact approved files while respecting pricing confidentiality and practical source-format requirements.
## 3. Scope
Upload/output allowlists and resource limits; scanner/renderer selection; canonical manifest/fingerprint; S3/email adapter data policies; quota/reservation/grace/suspension behavior. Later payment/OCR/model providers remain separately gated.
## 4. Out of Scope
Actual provider integration, assuming ZIP uploads because output packages use ZIP, selecting commercial prices without owner.
## 5. Technical Requirements
Define immutable storage version binding, signed URL residual-access window, safe previews, package format versioning, quota units and failed-job reservation release. Include tenant suspension exceptions explicitly.
## 6. Proposed Implementation Approach
Review representative synthetic tender files; compare scanning/rendering safety and fidelity; define deterministic hash fixtures; agree plan-limit semantics and provider responsibilities with security/product/commercial owners.
## 7. Dependencies
None; decisions must be signed before storage/quota implementation.
## 8. Acceptance Criteria
- Given identical candidate/renderer/profile, then canonical fingerprint input is unambiguous.
- Given scanner unavailable, then policy leaves files quarantined.
- Given quota exhaustion or suspended tenant, then permitted/denied operations and UI errors are explicit.
## 9. Testing Requirements
Approved file corpus and malformed cases, hash vectors, quota race scenarios and retention handoff review.
## 10. Definition of Done
Versioned policies and fixtures, owners and provider/format decisions recorded with approval status.
## 11. References
[Documents PR-SH-003](../../product/03_PRD.md), [storage](../../technical_design/15_DOCUMENT_STORAGE_DESIGN.md), [integrations](../../technical_design/16_INTEGRATION_ARCHITECTURE.md), [document plan](../../implementation/11_DOCUMENT_STORAGE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Unapproved formats must not be accepted optimistically; operator-assigned plans still need enforceable rules.
