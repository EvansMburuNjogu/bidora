# BID-ARC-001 — Release scope decision packet

Prepared: 2026-10-17. Status: **Option A approved and canonical decision synchronized; remaining reviewer steps pending**.

Execution authorization: prepare the first ticket's design artifacts and record the user's subsequent scope approval. Option A's release-specific boundaries are approved; application development is not authorized. Governing ticket: [BID-ARC-001](architecture/BID-ARC-001.md). The canonical [BID-ADR-006 register](../technical_design/27_ARCHITECTURE_DECISIONS.md) now records Approved, synchronized under the user's subsequent instruction to proceed with the best next step.

## 1. Decision requested

**Approved selection: Option A — a manual external-tender increment**, separately accepted from the full product MVP. Release label: **External Tender Pilot Increment**. Avoid claiming that this increment satisfies all BRD/PRD MVP acceptance or the full R2 capability list.

The increment lets a verified bid team create an isolated workspace, capture verified external tender facts, prepare evidence and responses, obtain human approval, generate an immutable package, record external submission proof and track reported outcomes.

| Option | Scope and consequence | Recommendation |
|---|---|---|
| A — Manual external increment | Deliver deterministic external workflow first; defer AI/search, marketplace and commercial automation. Explicitly narrower than PRD MVP-A and roadmap R2 details. Additional approvals still gate policies and implementation. | Recommended for an independently usable bounded release |
| B — Complete PRD MVP-A first | Add initial AI extraction/grounded analysis, approved-source Deep Search, profile/collaboration and billing-foundation coverage before acceptance. More provider, privacy, corpus and design work. Still not full BRD §10/PRD §8 acceptance. | Choose if these are necessary for first customer value |
| C — Full BRD/PRD MVP acceptance first | Complete both external and platform procurement journeys, all portal boundaries, sealing/opening and applicable AI controls before declaring MVP accepted. Requires future schemas/contracts/runtime children and independent crypto review. | Preserves original broad acceptance without a separate early release |

No duration, budget or commercial commitment is implied by these alternatives.

## 2. Source authority and conflicts

[Product master §3](../product/00_MASTER_INDEX.md) orders approved business/legal decisions above BRD, PRD, SRD, domain/design/security/QA, then roadmap/delivery plan, then tickets/code. Therefore R2/W07 sequencing does **not** override broader business acceptance.

| Source | Actual obligation or direction | Resolution proposed under Option A |
|---|---|---|
| [BRD §10.1](../product/02_BRD.md) | Procurement author → publish → sealed receipt → open → evaluate → recommendation/outcome | Preserve full obligation; defer to M8 and its runtime children |
| BRD §10.2 | External discovery/import, analysis, preparation, approval, package, evidence, outcome | Accept manual interpretation and verified import for this increment only; automated discovery/analysis remains M7 |
| BRD §10.3 | Platform tender response with verifiable acknowledgement | Defer to M8; external proof is not that acknowledgement |
| BRD §10.4 | Isolation/auth/sealing/audit/backup/restore tests | Isolation/auth/audit/backup/restore mandatory for increment; sealing capability absent and unaccepted until M8 |
| BRD §10.5 | Grounded, reviewable AI without final decision authority | No AI outputs in increment; requirement retained for M7/M8 AI capabilities, not marked passed |
| [PRD §7 MVP-A](../product/03_PRD.md) | Includes initial AI, basic Deep Search, billing foundation, profile and collaboration | Explicit partial delivery, not complete MVP-A; detailed exclusions below |
| PRD §7 MVP-B/C | Closed-loop procurement and advanced AI | Deferred, not removed; ARC-010 and M8/M7 child packets |
| PRD §8 | Three portals, both full journeys, deletion/restore, pre-opening denial, search/AI isolation and citations | Only implemented external/administrative boundaries accepted; other portals/features and lifecycle omissions remain outstanding |
| [Roadmap R2](../product/16_PRODUCT_ROADMAP.md) | External workflow, multiple import channels, full capability profile, tasks/dependencies | Supports sequencing, but Option A narrows import/profile/task coverage; do not label all R2 complete |
| [W07/S13–S14](../product/17_DELIVERY_PLAN_WAVES_SPRINTS.md) | Includes weighted assessment, dependencies, bulk assignment/task views and full external flow | Core human decision and requirement ownership first; assessment automation/advanced task UI explicitly deferred |

## 3. Release-specific acceptance matrix

These are proposed obligations for Option A, not passed tests. Ticket shorthand below uses the BID- prefix; links to every ticket are in the [master](02_MASTER_BACKLOG.md).

| Operation | Required acceptance evidence | Owners / planned coverage |
|---|---|---|
| Verified workspace onboarding | Verified identity, current membership, resumable provisioning and two isolated tenant databases; no forged context or default tenant | Security/engineering; AUTH-001/002, TEN-001–003, FE-001, SEC-002 |
| Manual tender capture | Source reference and exact attachments, original/normalized deadline with timezone, unknown facts visible, human verification, duplicate/conflict handling | Product/bid practitioner; API-003, DB-004, BE-002, FE-002 |
| Bid/No-Bid | Explicit human decision and rationale; no invented weighted scoring or automatic recommendation | Product; ARC-003, BE-002, FE-002 |
| Vault and evidence | Clean immutable document versions, verification/expiry, authorized access and exact-version evidence linking; scanner failure stays quarantined | Security/product; DOC-001/002, FE-003 |
| Requirements/readiness | Assigned requirements with due dates, evidence and deterministic blockers; unsupported waiver cannot bypass mandatory requirements | Product/procurement; ARC-003, BE-003, FE-004 |
| Responses/pricing | Versioned technical content and decimal/currency financial lines; technical-only users never receive pricing; stale saves rejected | Product/security; BE-004, FE-005 |
| Review/approval | Human decisions bind exact immutable candidate; edit/revocation races invalidate applicability without erasing history | Procurement/product; BE-005, FE-006 |
| Package | Approved output format, manifest and exact bytes tied to candidate; idempotent generation and storage/DB reconciliation | Product/engineering; ARC-008, DOC-003 |
| External submission record | User performs submission outside Bidora; clean proof references package, reported action time differs from capture time; confirmation policy approved | Product/procurement; ARC-003, BE-006, FE-007 |
| Outcome | Approved reported-outcome transitions and corrections preserve prior history; no inferred award | Product; ARC-003, BE-007, FE-008 |
| Operational acceptance | Actual E2E, isolation, accessibility/mobile, security, load, audit, measured backup/restore, compatible deployment and named support ownership | QA/security/operations; QA-002–004, SEC-003, OPS-002/003, OBS-002, REL-001–003 |

No mock API, manual database repair, unsupported exception or AI verdict may satisfy these criteria. Policies on approval roles, self-approval, evidence confirmation, formats and retention remain separate decisions; Option A does not select defaults.

## 4. Explicit exclusions and preserved future work

| Exclusion / constrained subset | Preserved source scope | Future gate / accountable role |
|---|---|---|
| Automatic URL fetching, email ingestion, OCR and AI extraction; URL may be stored only as provenance | PR-TP-004/005; R2 multi-channel import | M7, ARC-010, INT-002, AI-001–004; product/security |
| Deep Search, approved connectors, matching/watchlists/alerts | PR-TP-003; MVP-A; R3 | M7, ARC-010/AI-001; extra runtime children for capabilities outside current AI tickets; product |
| AI drafting/proposal/evaluation assistants | PR-TP-010; PR-PT-003/010; MVP-C | M7/M8, ARC-010 and capability-specific children; product/security |
| Full company capability profile and reusable knowledge library | PR-TP-001/002; R2 | Core identity/vault only now; ARC-010 allocates M7 expansion children; product |
| Weighted Bid/No-Bid assessment, general task dependencies, bulk/task views, delegation and full proposal collaboration | W07 S13; PR-SH-004/005; MVP-A | Core human decision/requirement ownership/revisions now; ARC-010 allocates M7 children; product/procurement |
| Automated invoices/payments and complete billing UI | PR-PA-002; PR-SH-009; MVP-A billing foundation | Operator-assigned plans/quotas only if ARC-008/ADR-015 approved; INT-003 design and M7 runtime children; commercial owner |
| Full procurement portal/public catalogue/authoring/publication/addenda | PR-PT-001–006; BRD §10.1; PRD §8 | M8, API-009/DB-008/BE-008/009 plus UI/runtime children; procurement |
| Platform submission, sealing, opening, evaluation/award/handover | PR-TP-012; PR-PT-007–011; BRD §10.1/3 | M8, ARC-009, BE-010–013, QA-005 and runtime children; procurement/security |
| Full platform-admin/support access/enterprise SSO | PR-PA-001/003/005; SR-IAM-004 | Minimal authorized operational paths only; private support denied by default; ARC-010 allocates M7/M8 children; security/operations |
| General tenant deletion/erasure automation, advanced analytics and country packs | PRD §8 lifecycle tests; DATA-008; later R8 | ARC-010 allocates explicit future design/runtime children. Legal erasure/retention obligations cannot be deferred by this ticket; legal/privacy review must approve any lawful interim process before pilot data |

Future milestone assignment is a planning gate, not a claim of detailed runtime coverage. ARC-010 must create bounded children where coverage is currently umbrella-level.

## 5. Milestones and traceability

The existing [milestone register](05_RELEASE_MILESTONES.md) remains authoritative for local M* IDs:

- M0: approved scoped decisions and threat/design baseline.
- M1: real Go/web foundation.
- M2: authenticated isolated workspace and negative isolation gate.
- M3: verified external tender/bid and secure documents.
- M4: complete scoped external journey with staging/security/QA/restore acceptance.
- M5: controlled pilot, authorized data terms and feedback disposition.
- M6: human-authorized production promotion.
- M7: separately approved search/AI/commercial and other expansion packets.
- M8: future closed-loop procurement after design, runtime children and independent acceptance.

These numbers are not product releases R0–R8. [Roadmap mapping](04_DELIVERY_ROADMAP.md) preserves all BID-T000–017 planning tasks: this ticket elaborates BID-T000, does not execute BID-T001 or any runtime task. External operations map to BID-T007–014, acceptance to BID-T015, future AI to BID-T016 and closed-loop design to BID-T017.

## 6. Naming and ADR alias treatment

New artifacts use **Bidora** per approved BID-ADR-001. Product documents and the combined TenderOS specification remain unchanged. Branding migration is a separate authorized change and must keep combined and individual sources consistent.

Never use an unqualified legacy ADR number as authority. For example:

| Qualified source | Meaning | Canonical mapping |
|---|---|---|
| `docs/product/08_HLD.md` ADR-001 | Modular monolith | BID-ADR-002 |
| `docs/product/22_GLOSSARY_ASSUMPTIONS_DECISIONS.md` ADR-001 | Repository layout | BID-ADR-005 |
| `docs/product/08_HLD.md` ADR-006 | Storage | BID-ADR-004 / BID-ADR-017 |
| `docs/product/22_GLOSSARY_ASSUMPTIONS_DECISIONS.md` ADR-006 | Broker | BID-ADR-004 |

Use the complete [canonical alias table](../technical_design/27_ARCHITECTURE_DECISIONS.md) for other legacy references. Scope acceptance does not approve path, identity, migration tooling, providers or sealing decisions.

## 7. Reviewer walkthroughs — not executed acceptance tests

**External scenario:** Bid manager creates a private workspace, manually verifies a tender and its deadline, records Bid, uploads clean certificates, resolves mandatory requirements, prepares restricted responses, requests review, generates the approved package, submits outside Bidora, captures/has proof confirmed and records the outcome. Reviewers confirm this is useful without automated discovery/AI and approve or reject the listed profile/task/import reductions.

**Failure scenario:** Certificate expires or response changes after approval. Readiness/package applicability must reject stale evidence; historical approval remains. Missing proof cannot silently become submitted. Business owners must specify any permitted exceptions in ARC-003; this packet invents none.

**Closed-loop scenario:** Procurement user attempts to publish a platform tender or open a sealed response. These capabilities are absent in Option A, not partially accepted. BRD §10.1/3 and PRD §8 platform acceptance stay open until M8 runtime/security/UAT evidence exists.

**Privacy scenario:** Pilot customer requests erasure while submitted evidence is subject to retention. A reviewed lawful process and owner must exist before handling real pilot data; a future automation ticket alone is insufficient.

Reviewer outcomes: **not yet recorded**. No UAT, application, security or restore tests ran for this packet.

## 8. Approval record and remaining action

| Decision field | Current record |
|---|---|
| Selected alternative | **Option A — approved** with the inclusions/exclusions in this packet |
| Approver identity / capacity | Requesting user in this conversation; personal name and formal product-owner title not supplied |
| Product decision and date | User approval recorded 2026-10-17; no amendments stated |
| Procurement/domain reviewer | Not assigned; walkthrough pending |
| Security/privacy/operations review | Pending for relevant scope and pilot safeguards |
| Evidence | Explicit user response to this packet: **“I approve option A”** |
| Source scope affected | BRD §10; PRD §§7–8; roadmap R2; W07 S13–S14 |
| Implementation authorization | Not granted by this packet |
| BID-ARC-001 Done | No — scope approval and canonical decision synchronization recorded; reviewer evidence remains outstanding |

Approval effect: accept the manual external-tender increment and its explicit exclusions; preserve full BRD/PRD acceptance for later releases. This does not approve identity, review/exception rules, providers, formats, privacy obligations, or application implementation.

Synchronization completed: BID-ADR-006 and implementation strategy now reflect Option A approval. Next steps: obtain the outstanding reviewer dispositions; independently prepare ARC-002 repository/toolchain decisions. Any product-source addendum requires separate authorization; do not silently rewrite BRD/PRD acceptance. The ticket remains Proposed pending reviewer closure. Dependent work is not automatically Ready; its remaining prerequisites still apply.
