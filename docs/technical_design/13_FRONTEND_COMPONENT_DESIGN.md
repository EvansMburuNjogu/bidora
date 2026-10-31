# Frontend component design

Sources: [frontend §§5,9–15](../product/12_FRONTEND_UX_DESIGN.md), [template adoption](../product/20_FRONTEND_TEMPLATE_ADOPTION.md). Decisions [005, 017](27_ARCHITECTURE_DECISIONS.md). Execution: [frontend](../implementation/10_FRONTEND_IMPLEMENTATION.md).

## Component boundaries

Design-system primitives own visual states and accessibility, not domain permissions. Feature components receive typed DTOs/action hints and invoke feature orchestration hooks. Backend errors remain authoritative. Theme tokens cover semantic status, focus, spacing, typography and density; AI/advisory styling cannot resemble approved evidence.

| Component | Inputs / outputs | Required behavior and test |
|---|---|---|
| WorkspaceSwitcher | authorized memberships, context generation; selection event | Abort pending requests, clear tenant cache, confirm unsaved changes; stale response cannot populate new workspace |
| DomainDataTable | typed rows, cursor, allowed sorts/actions | Server pagination/filtering; stable focus and semantic headers; empty/denied states; no raw UUID input controls |
| DocumentUploader | policy limits, authorized upload session; completed document ID | Multipart progress/resume, quarantine/scan state, no preview before clean; lost completion response retry |
| EvidencePicker | authorized document versions, requirement source | Exact version selection, expiry/verification badges, no inaccessible filename leakage |
| ComplianceMatrix | requirements, action hints, readiness summary | Owner/status/evidence side panel; optimistic cosmetic changes only; rollback on rejected transition; keyboard row navigation |
| ResponseEditor | revision, permitted content format, expected version | Visible save/unsaved/conflict state; no silent overwrite; sanitized display; test malicious pasted HTML |
| FinancialSchedule | decimal-string lines/currency | Fixed approved rounding rules; no float-derived authoritative totals; pricing permission independent of technical editor |
| ApprovalPanel | immutable candidate, assigned step, assurance state | Display precise version and consequences; require confirmation; expired authority refresh; cannot approve stale snapshot |
| JobProgress | job DTO and status URL | Poll with backoff/cancel on navigation; unknown progress shown honestly; distinguish queued/failed/succeeded |
| PackageManifest | permitted files/hashes and candidate | Restricted content filtered; downloadable package inherits classification; checksum details available without secret object keys |
| EvidenceTimeline | immutable facts and correction links | Show reported time versus server capture time; corrections preserve history; accessible chronology |
| DomainErrorSummary | stable code, safe field errors, trace ID | Focus summary, associate fields, actionable refresh/retry; never render server strings as HTML |

## Composition example

SubmissionPage composes ReadinessPanel → ApprovalPanel → PackageJobPanel → PackageManifest → ExternalEvidenceForm → EvidenceTimeline. Each panel queries its own approved contract but shares tenant/bid/candidate identity. A ready-looking panel cannot bypass a rejected server transition. Financial details are not loaded then hidden with CSS.

## Template adoption and evidence

Inventory template assets/licenses/scripts before execution. Map components to actual Bidora operations; remove mock users/charts/analytics. Test focus restore, keyboard upload, long filenames, error recovery, screen-reader async announcements, 200% zoom and mobile evidence capture. Record screenshots as UX evidence, not proof of authorization. See [state design](14_STATE_MANAGEMENT_DATA_FLOW.md).
