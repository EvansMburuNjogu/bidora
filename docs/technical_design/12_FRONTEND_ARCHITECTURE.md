# Frontend architecture and screens

Sources: [frontend specification](../product/12_FRONTEND_UX_DESIGN.md), PR-TP/PR-SH and UX-001–008 in [PRD](../product/03_PRD.md). Decisions [005–007, 016](27_ARCHITECTURE_DECISIONS.md). Execution: [frontend plan](../implementation/10_FRONTEND_IMPLEMENTATION.md).

## Routes and ownership

`<frontend-root>/src/app` has public/auth route groups and explicit `/platform`, `/procurement`, `/tenderpreneur` paths. Route groups do not provide security. Layouts obtain `/v1/me` through request-scoped server fetches; Go authorizes every operation. Procurement routes are not placeholder features in the external MVP. Server Components render initial authorized data and layouts; Client Components own forms, matrix interaction, uploads and polling. No direct DB/internal KMS/AI calls from Next.js.

## Major screen contracts

All screens require loading, empty, validation, forbidden, concealed-not-found, network, stale/conflict and server-error behavior as applicable. Preserve permitted input after failure; never persist pricing or session tokens in localStorage.

| Screen / route / persona | Authorization and data/API dependencies | Actions, validation and acceptance |
|---|---|---|
| Login/verify/recovery/MFA `/login`, `/verify-email`, `/recovery`, `/mfa` | Public challenge or session; identity contracts | Submit once, generic account errors, accessible error summary; keyboard and challenge-expiry/replay tests |
| Workspace onboarding `/onboarding/organization`, `/onboarding/provisioning` | Verified identity, create entitlement; workspaces/job APIs | Capture organization/locale/zone; show durable progress, safe retry; never label ACTIVE from optimistic UI |
| Workspace/team `/tenderpreneur/users` | Member-admin capability; invitation/membership APIs | Grantable role select, last-admin error; unauthorized roles absent and API denial tested |
| Opportunity capture `/tenderpreneur/opportunities/new` | Opportunity manage; manual/verify-facts APIs | Source link/document, explicit zone and unknown facts; no OCR result fabricated; validate ambiguous deadline before verify |
| Opportunity detail `/tenderpreneur/opportunities/[id]` | Opportunity scope; source/version query | Verify facts, inspect source, create bid; duplicate and stale source conflict tested |
| Bid overview `/tenderpreneur/bids/[id]` | Bid read; detail/decision/readiness | Human Bid/No-Bid, owner and blockers, deadline/source version; no decorative fake counters |
| Vault `/tenderpreneur/compliance-vault` | Document list/classification; upload/document APIs | Upload, inspect versions, verify, archive; quarantine visible, no premature preview |
| Requirements `/tenderpreneur/bids/[id]/requirements` | Bid matrix scope; requirement/evidence APIs | Assign, attach exact evidence, review status; unsupported SATISFIED rejected; keyboard table and mobile detail view |
| Responses `/tenderpreneur/bids/[id]/response` and `/financial-response` | Separate edit/read pricing capability | Versioned save, compare/refresh conflict; decimal/zone-aware controls; denied pricing never included in initial payload |
| Review `/tenderpreneur/bids/[id]/reviews` | Assigned reviewer/approver; candidate/decision APIs | Snapshot summary, changes requested, approve; show stale candidate and authority expiry; no auto-approval |
| Submission `/tenderpreneur/bids/[id]/submission` | Package and evidence capabilities; readiness/jobs/evidence | Generate approved package, poll, download, capture proof, confirm and mark submitted; distinguish external action from internal record |
| Outcome `/tenderpreneur/bids/[id]/outcome` | Outcome capability; outcome/history APIs | Select approved outcome, record reasons/proof; corrections append rather than overwrite |
| Operator tenant detail `/platform/tenants/[id]` | Platform metadata capability; registry/job APIs | Provision/suspend status and reasoned actions; private tender titles/pricing never shown by default |

Read-only detail/candidate/history endpoints not fully specified in the API cards require explicit contract additions before screen implementation; never invent frontend mock APIs.

## Accessibility and responsiveness

WCAG 2.2 AA target. Every form has labels, field associations and focusable error summary. Async scan/job changes use restrained live regions. Dialog focus is trapped/restored. Matrices use semantic headers; virtualized grids require screen-reader/keyboard proof, not only speed. Mobile essentials include source inspection, upload, approvals and evidence capture; complex tables offer accessible detail panels. Test 200% zoom, long names, keyboard-only flow, reduced motion and non-color status cues.

## Acceptance

Use real backend contracts for accepted screens, Playwright multi-role flows, component tests and axe plus manual screen-reader checks. Template license/asset audit is required before adoption; unavailable template must not block a separately approved product-owned design baseline.
