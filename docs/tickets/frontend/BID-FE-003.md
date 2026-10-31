# BID-FE-003 — Build secure uploader, exact-version vault and permitted previews

## Metadata
- Ticket ID: BID-FE-003
- Epic: E-FE
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-FE-001, BID-DOC-002, BID-API-004
- Blockers: Approved format/preview policy
- Milestone: M3

## 1. Objective
Expose safe document processing and reusable evidence versions without premature trust.
## 2. Business Context
Teams must see whether a certificate is clean, current and verified before using it in a bid.
## 3. Scope
Uploader progress/retry, vault filters, document/version detail, quarantine/rejected states, verification/expiry, safe preview and authorized download.
## 4. Out of Scope
OCR/AI, active-content rendering, public object URLs or localStorage business source.
## 5. Technical Requirements
API-004 typed contracts; no preview before CLEAN; preserve one upload identity across ambiguous completion retry; exact version picker; classification-filtered metadata; accessible async announcements.
## 6. Proposed Implementation Approach
Compose uploader/job panel; bind permitted metadata/actions; render only approved sanitized preview type; add version and expiry UX; test interruptions and context switches.
## 7. Dependencies
[Shell](BID-FE-001.md), [documents](../documents/BID-DOC-002.md), [contracts](../backend_contracts/BID-API-004.md).
## 8. Acceptance Criteria
- Given quarantined/rejected file, then UI never offers usable evidence/preview.
- Given transfer interruption, then retry does not create duplicate document version.
- Given permission revoked, then preview/download closes or fails safely without cached foreign data.
## 9. Testing Requirements
Upload browser tests, malformed/oversize UX, keyboard file picker, scan announcements, expired version and tenant-switch races.
## 10. Definition of Done
Real storage/scan-backed vault workflow with accessibility and failure evidence.
## 11. References
[PR-SH-003](../../product/03_PRD.md), [components](../../technical_design/13_FRONTEND_COMPONENT_DESIGN.md), [document plan](../../implementation/11_DOCUMENT_STORAGE_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Browser progress measures transfer, not scan completion or business availability.
