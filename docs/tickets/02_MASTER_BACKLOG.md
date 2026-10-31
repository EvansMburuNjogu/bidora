# Master engineering backlog

110 tickets; ticket statuses do not certify verification or acceptance. **8 Proposed, 89 Blocked, 12 IMPLEMENTED_NOT_VERIFIED; 0 Ready, 1 In Review, 0 Done.** Explicit implementation authorization is an additional global prerequisite. Titles below are compact display labels; individual files define scope and acceptance.

**Active Phase 1 reconciliation (2026-10-20):** the historical 110-row status totals below have not been rewritten for every subsequently implemented child slice. For the current backend-only execution boundary, the canonical remaining-work count is **25 packets**: 13 functional umbrellas, 10 Option A verification packets and 2 Phase 1 acceptance tickets. See [Phase 1 gap audit §2](14_PHASE_1_PORTAL_GAP_AUDIT.md#2-reconciled-active-backend-count--2026-10-20). This count excludes three unauthorized frontend tickets, future procurement/exchange work, undefined Procurement Portal scope and deployment/UAT work outside the named release gate. Do not derive active progress by subtracting the historical status totals from 110.

Priority: P0 foundation/integrity dependency, P1 scoped release completion, P2 later capability. Complexity S/M/L is relative scope/risk, not a delivery estimate. E-* identifies the [epic](01_EPICS.md); M* identifies the [evidence milestone](05_RELEASE_MILESTONES.md), not a source release ID. Dependencies are hard prerequisite ticket IDs, not substitutes for policy approvals.

| ID / ticket | Title | Epic | Priority | Size | Dependencies | Status | Milestone |
|---|---|---|---|---|---|---|---|
| [BID-ARC-001](architecture/BID-ARC-001.md) | Approve external scope/source alignment | E-ARC | P0 | M | None | Proposed | M0 |
| [BID-ARC-002](architecture/BID-ARC-002.md) | Approve root/toolchain/agent conventions | E-ARC | P0 | S | None | Proposed | M0 |
| [BID-ARC-003](architecture/BID-ARC-003.md) | Approve bid/review/evidence policies | E-ARC | P0 | M | None | Proposed | M0 |
| [BID-ARC-004](architecture/BID-ARC-004.md) | Close schema/backend design gaps | E-ARC | P0 | M | BID-ARC-003, BID-ARC-005 | Blocked | M0 |
| [BID-ARC-005](architecture/BID-ARC-005.md) | Approve identity/authorization | E-ARC | P0 | M | None | Proposed | M0 |
| [BID-ARC-006](architecture/BID-ARC-006.md) | Approve screens/design system | E-ARC | P1 | M | None | Proposed | M0 |
| [BID-ARC-007](architecture/BID-ARC-007.md) | Approve operations/privacy/QA | E-ARC | P1 | M | None | Proposed | M0 |
| [BID-ARC-008](architecture/BID-ARC-008.md) | Approve formats/providers/quotas | E-ARC | P0 | M | None | Proposed | M0 |
| [BID-ARC-009](architecture/BID-ARC-009.md) | Resolve sealed time/key-release design | E-ARC | P2 | L | None | Proposed | M8 |
| [BID-ARC-010](architecture/BID-ARC-010.md) | Close selected Phase 1 coverage gaps | E-ARC | P1 | M | BID-ARC-001 | Blocked | M7 |
| [BID-ARC-011](architecture/BID-ARC-011.md) | Cell-ready CBA and Hexagonal DDD boundaries | E-ARC | P0 | M | BID-ARC-010 | IMPLEMENTED_NOT_VERIFIED | M7 |
| [BID-FND-001](foundation/BID-FND-001.md) | Runnable Go health boundary | E-FND | P0 | M | BID-ARC-002, BID-API-001, BID-FND-003 | Blocked | M1 |
| [BID-FND-002](foundation/BID-FND-002.md) | Next.js real API smoke view | E-FND | P0 | M | BID-FND-001, BID-ARC-006 | Blocked | M1 |
| [BID-FND-003](foundation/BID-FND-003.md) | Validated local config/services | E-FND | P0 | M | BID-ARC-002, BID-ARC-007 | Blocked | M1 |
| [BID-FND-004](foundation/BID-FND-004.md) | Local-only PostgreSQL foundation child | E-FND | P0 | M | BID-FND-005 | Blocked | M1 |
| [BID-FND-005](foundation/BID-FND-005.md) | Docker-free local resource validation | E-FND | P0 | S | None | In Review | M1 |
| [BID-API-001](backend_contracts/BID-API-001.md) | OpenAPI/health/error conventions | E-API | P0 | S | BID-ARC-002 | Blocked | M1 |
| [BID-API-002](backend_contracts/BID-API-002.md) | Identity/membership/workspace contracts | E-API | P0 | M | BID-API-001, BID-ARC-005, BID-ARC-008 | Blocked | M2 |
| [BID-API-003](backend_contracts/BID-API-003.md) | Opportunity/bid/compliance contracts | E-API | P0 | M | BID-API-001, BID-ARC-001, BID-ARC-003 | Blocked | M3 |
| [BID-API-004](backend_contracts/BID-API-004.md) | Upload/document/job contracts | E-API | P0 | M | BID-API-001, BID-ARC-008 | Blocked | M2 |
| [BID-API-005](backend_contracts/BID-API-005.md) | Response/candidate/review contracts | E-API | P0 | M | BID-API-003, BID-ARC-004 | Blocked | M4 |
| [BID-API-006](backend_contracts/BID-API-006.md) | Package/proof/outcome contracts | E-API | P1 | M | BID-API-004, BID-API-005, BID-ARC-003 | Blocked | M4 |
| [BID-API-007](backend_contracts/BID-API-007.md) | Event/effect-recovery contracts | E-API | P0 | M | BID-API-001, BID-ARC-004 | Blocked | M2 |
| [BID-API-008](backend_contracts/BID-API-008.md) | Generated clients/compatibility gates | E-API | P1 | M | BID-API-002, BID-FND-002 | Blocked | M2 |
| [BID-API-009](backend_contracts/BID-API-009.md) | Future procurement contract packets | E-API | P2 | L | BID-API-001, BID-ARC-009, BID-ARC-010 | Blocked | M8 |
| [BID-API-010](backend_contracts/BID-API-010.md) | Shared portal/platform admin contracts | E-API | P0 | M | BID-API-001, BID-ARC-005, BID-ARC-010 | Blocked | M7 |
| [BID-API-011](backend_contracts/BID-API-011.md) | Plans/subscriptions/PesaPal contracts | E-API | P0 | M | BID-API-001, BID-ARC-010, BID-INT-003 | Blocked | M7 |
| [BID-API-012](backend_contracts/BID-API-012.md) | Knowledge/Tender Assistant contracts | E-API | P0 | M | BID-API-001, BID-AI-001, BID-ARC-010 | Blocked | M7 |
| [BID-DB-001](database/BID-DB-001.md) | Migration sets and least-privilege roles | E-DB | P0 | M | BID-ARC-004, BID-FND-003 | Blocked | M2 |
| [BID-DB-002](database/BID-DB-002.md) | Control identity/registry persistence | E-DB | P0 | M | BID-DB-001, BID-API-002 | Blocked | M2 |
| [BID-DB-003](database/BID-DB-003.md) | Common/audit/outbox/inbox persistence | E-DB | P0 | M | BID-DB-001, BID-API-007 | Blocked | M2 |
| [BID-DB-004](database/BID-DB-004.md) | Opportunity/bid/requirement persistence | E-DB | P0 | M | BID-DB-003, BID-DB-005, BID-API-003 | Blocked | M3 |
| [BID-DB-005](database/BID-DB-005.md) | Document/upload/retention persistence | E-DB | P0 | M | BID-DB-003, BID-API-004, BID-ARC-007 | Blocked | M2 |
| [BID-DB-006](database/BID-DB-006.md) | Response/candidate/approval persistence | E-DB | P0 | M | BID-DB-004, BID-API-005 | Blocked | M4 |
| [BID-DB-007](database/BID-DB-007.md) | Package/proof/outcome persistence | E-DB | P1 | M | BID-DB-006, BID-API-006 | Blocked | M4 |
| [BID-DB-008](database/BID-DB-008.md) | Future procurement/exchange schema design | E-DB | P2 | L | BID-ARC-009, BID-ARC-010, BID-API-009 | Blocked | M8 |
| [BID-BE-001](backend/BID-BE-001.md) | Transaction/idempotency/error primitives | E-BE | P0 | M | BID-FND-001, BID-DB-003, BID-OBS-001 | IMPLEMENTED_NOT_VERIFIED | M2 |
| [BID-BE-002](backend/BID-BE-002.md) | Verify tender/create human-owned bid | E-BE | P0 | M | BID-DB-004, BID-API-003, BID-SEC-002 | VERIFIED_TECHNICALLY | M3 |
| [BID-BE-003](backend/BID-BE-003.md) | Evidence/assignments/readiness | E-BE | P0 | M | BID-BE-002, BID-DOC-002, BID-API-003 | VERIFIED_TECHNICALLY | M4 |
| [BID-BE-004](backend/BID-BE-004.md) | Technical revisions/restricted pricing | E-BE | P0 | M | BID-BE-003, BID-DB-006, BID-API-005 | VERIFIED_TECHNICALLY | M4 |
| [BID-BE-005](backend/BID-BE-005.md) | Immutable review/human approval | E-BE | P0 | M | BID-BE-004, BID-API-005 | VERIFIED_TECHNICALLY | M4 |
| [BID-BE-006](backend/BID-BE-006.md) | External proof/mark-submitted | E-BE | P1 | M | BID-DOC-003, BID-API-006 | VERIFIED_TECHNICALLY | M4 |
| [BID-BE-007](backend/BID-BE-007.md) | Outcome corrections/history | E-BE | P1 | M | BID-BE-006, BID-API-006 | VERIFIED_TECHNICALLY | M4 |
| [BID-BE-008](backend/BID-BE-008.md) | Design requisition/tender authoring | E-BE | P2 | M | BID-DB-008, BID-ARC-010 | Blocked | M8 |
| [BID-BE-009](backend/BID-BE-009.md) | Design publication/clarification/addenda | E-BE | P2 | M | BID-BE-008, BID-API-009 | Blocked | M8 |
| [BID-BE-010](backend/BID-BE-010.md) | Design evaluation/score locks | E-BE | P2 | M | BID-DB-008, BID-API-009, BID-BE-013 | Blocked | M8 |
| [BID-BE-011](backend/BID-BE-011.md) | Design award/notices/handover | E-BE | P2 | M | BID-BE-010, BID-API-009 | Blocked | M8 |
| [BID-BE-012](backend/BID-BE-012.md) | Design sealed accept/replace/withdraw | E-BE | P2 | M | BID-ARC-009, BID-DB-008, BID-BE-009, BID-API-009 | Blocked | M8 |
| [BID-BE-013](backend/BID-BE-013.md) | Design quorum opening/release recovery | E-BE | P2 | M | BID-BE-012 | Blocked | M8 |
| [BID-BE-014](backend/BID-BE-014.md) | Typed platform settings/countries | E-BE | P0 | M | BID-AUTH-003, BID-API-010, BID-DB-002 | Blocked | M7 |
| [BID-BE-015](backend/BID-BE-015.md) | Approved organization profile/knowledge | E-BE | P0 | M | BID-AUTH-002, BID-API-012, BID-DOC-002, BID-DB-003 | Blocked | M7 |
| [BID-BE-016](backend/BID-BE-016.md) | Actionable Phase 1 dashboards | E-BE | P1 | M | BID-BE-014, BID-TEN-004, BID-AUTH-004, BID-INT-004, BID-OBS-002 | Blocked | M7 |
| [BID-BE-017](backend/BID-BE-017.md) | Versioned letterhead proposal exports | E-BE | P1 | M | BID-BE-004, BID-BE-015, BID-API-012, BID-DOC-002 | Blocked | M7 |
| [BID-AUTH-001](authentication/BID-AUTH-001.md) | Verified identity/session/recovery/MFA | E-AUTH | P0 | L | BID-DB-002, BID-BE-001, BID-API-002, BID-INT-001, BID-SEC-001 | Blocked | M2 |
| [BID-AUTH-002](authentication/BID-AUTH-002.md) | Membership/scopes/context/invitations | E-AUTH | P0 | M | BID-AUTH-001, BID-API-002 | Blocked | M2 |
| [BID-AUTH-003](authentication/BID-AUTH-003.md) | Portal type/fixed platform authority | E-AUTH | P0 | M | BID-AUTH-001, BID-API-010, BID-DB-002, BID-SEC-001 | Blocked | M7 |
| [BID-AUTH-003C](authentication/BID-AUTH-003C.md) | Platform Administrator invitation lifecycle | E-AUTH | P0 | M | BID-AUTH-003, BID-ARC-011, BID-API-010 | IMPLEMENTED_NOT_VERIFIED | M7 |
| [BID-AUTH-003D](authentication/BID-AUTH-003D.md) | Govern Platform Owner invitation disposition | E-AUTH | P0 | M | BID-AUTH-003C, BID-ARC-011, BID-API-010 | IMPLEMENTED_NOT_VERIFIED | M7 |
| [BID-AUTH-003E](authentication/BID-AUTH-003E.md) | Fixed Platform Administrator role lifecycle | E-AUTH | P0 | M | BID-AUTH-003D, BID-ARC-011, BID-API-010 | IMPLEMENTED_NOT_VERIFIED | M7 |
| [BID-AUTH-003F](authentication/BID-AUTH-003F.md) | Execute pending Platform Owner role changes | E-AUTH | P0 | M | BID-AUTH-003E, BID-ARC-011, BID-API-010 | IMPLEMENTED_NOT_VERIFIED | M7 |
| [BID-AUTH-004](authentication/BID-AUTH-004.md) | Controlled tenant support access | E-AUTH | P1 | M | BID-AUTH-003, BID-API-010, BID-SEC-001 | Blocked | M7 |
| [BID-TEN-001](tenancy/BID-TEN-001.md) | Resumable isolated provisioning | E-TEN | P0 | M | BID-AUTH-002, BID-EVT-002, BID-DOC-001, BID-TEN-003 | Blocked | M2 |
| [BID-TEN-002](tenancy/BID-TEN-002.md) | Authorized bounded tenant router | E-TEN | P0 | M | BID-TEN-001, BID-AUTH-002 | Blocked | M2 |
| [BID-TEN-003](tenancy/BID-TEN-003.md) | Quota reservations/suspension | E-TEN | P1 | M | BID-AUTH-002, BID-DB-002, BID-API-002 | Blocked | M2 |
| [BID-TEN-004](tenancy/BID-TEN-004.md) | Plans/subscriptions/monthly Bid allowance | E-TEN | P0 | M | BID-TEN-003, BID-API-011, BID-AUTH-003 | Blocked | M7 |
| [BID-EVT-001](events_workers/BID-EVT-001.md) | Durable outbox/inbox dispatch | E-EVT | P0 | M | BID-BE-001, BID-API-007 | Blocked | M2 |
| [BID-EVT-002](events_workers/BID-EVT-002.md) | Authorized jobs/cancel/replay | E-EVT | P0 | M | BID-EVT-001, BID-AUTH-002, BID-API-004 | Blocked | M2 |
| [BID-DOC-001](documents/BID-DOC-001.md) | Private immutable S3 adapter | E-DOC | P0 | M | BID-DB-005, BID-BE-001, BID-SEC-001 | Blocked | M2 |
| [BID-DOC-002](documents/BID-DOC-002.md) | Quarantine/scan/verify/access | E-DOC | P0 | M | BID-DOC-001, BID-SEC-002, BID-EVT-002, BID-API-004 | Blocked | M3 |
| [BID-DOC-003](documents/BID-DOC-003.md) | Immutable recoverable package jobs | E-DOC | P1 | M | BID-BE-005, BID-DB-007, BID-DOC-002, BID-API-006 | Blocked | M4 |
| [BID-FE-001](frontend/BID-FE-001.md) | Auth/onboarding/context/team shell | E-FE | P0 | M | BID-API-008, BID-TEN-002, BID-SEC-002 | Blocked | M2 |
| [BID-FE-002](frontend/BID-FE-002.md) | Tender capture/bid workspace | E-FE | P0 | M | BID-FE-001, BID-BE-002, BID-API-003 | Blocked | M3 |
| [BID-FE-003](frontend/BID-FE-003.md) | Secure uploader/vault/preview | E-FE | P1 | M | BID-FE-001, BID-DOC-002, BID-API-004 | Blocked | M3 |
| [BID-FE-004](frontend/BID-FE-004.md) | Accessible compliance matrix | E-FE | P1 | M | BID-FE-002, BID-FE-003, BID-BE-003, BID-API-003 | Blocked | M4 |
| [BID-FE-005](frontend/BID-FE-005.md) | Versioned responses/finance editor | E-FE | P1 | M | BID-FE-004, BID-BE-004, BID-API-005 | Blocked | M4 |
| [BID-FE-006](frontend/BID-FE-006.md) | Exact-candidate review/decisions | E-FE | P1 | M | BID-FE-005, BID-BE-005, BID-API-005 | Blocked | M4 |
| [BID-FE-007](frontend/BID-FE-007.md) | Package/proof/submission guidance | E-FE | P1 | M | BID-FE-006, BID-BE-006, BID-API-006 | Blocked | M4 |
| [BID-FE-008](frontend/BID-FE-008.md) | Reported outcomes/correction history | E-FE | P1 | S | BID-FE-007, BID-BE-007, BID-API-006 | Blocked | M4 |
| [BID-FE-009](frontend/BID-FE-009.md) | Simplified AI-first Tenderpreneur portal | E-FE | P0 | M | BID-FE-008, BID-API-011, BID-API-012, BID-TEN-004, BID-AI-005, BID-BE-016, BID-BE-017 | Blocked | M7 |
| [BID-FE-010](frontend/BID-FE-010.md) | Platform users/settings console | E-FE | P0 | M | BID-FE-001, BID-API-010, BID-AUTH-003, BID-BE-014 | Blocked | M7 |
| [BID-FE-011](frontend/BID-FE-011.md) | Platform commercial/operations console | E-FE | P1 | M | BID-FE-010, BID-API-011, BID-BE-016, BID-INT-004, BID-AUTH-004, BID-SEC-004 | Blocked | M7 |
| [BID-INT-001](integrations/BID-INT-001.md) | Scoped notification delivery | E-INT | P1 | M | BID-EVT-001, BID-ARC-008 | Blocked | M2 |
| [BID-INT-002](integrations/BID-INT-002.md) | Safe approved-source connector | E-INT | P2 | M | BID-ARC-010, BID-EVT-002, BID-DOC-002 | Blocked | M7 |
| [BID-INT-003](integrations/BID-INT-003.md) | PesaPal billing/reconciliation design | E-INT | P0 | M | BID-ARC-010, BID-TEN-003 | Blocked | M7 |
| [BID-INT-004](integrations/BID-INT-004.md) | PesaPal payment reconciliation adapter | E-INT | P0 | M | BID-INT-003, BID-API-011, BID-TEN-004 | Blocked | M7 |
| [BID-AI-001](ai_search/BID-AI-001.md) | AI policy/schema/corpus approval | E-AI | P0 | M | BID-ARC-010, BID-ARC-008 | Blocked | M7 |
| [BID-AI-002](ai_search/BID-AI-002.md) | Authorized FTS/vector retrieval | E-AI | P0 | M | BID-AI-001, BID-DOC-002, BID-EVT-002, BID-BE-015 | Blocked | M7 |
| [BID-AI-003](ai_search/BID-AI-003.md) | Bounded parser/OCR/source coordinates | E-AI | P2 | M | BID-AI-001, BID-DOC-002, BID-EVT-002 | Blocked | M7 |
| [BID-AI-004](ai_search/BID-AI-004.md) | Grounded suggestions/human disposition | E-AI | P0 | M | BID-AI-002, BID-AI-003, BID-BE-003 | Blocked | M7 |
| [BID-AI-005](ai_search/BID-AI-005.md) | Natural-language Tender Assistant | E-AI | P0 | M | BID-AI-004, BID-API-012, BID-BE-015 | Blocked | M7 |
| [BID-SEC-001](security/BID-SEC-001.md) | Threat/control/test ownership | E-SEC | P0 | M | BID-ARC-005, BID-ARC-007, BID-ARC-008 | Blocked | M0 |
| [BID-SEC-002](security/BID-SEC-002.md) | Tenant isolation exposure gate | E-SEC | P0 | M | BID-QA-001, BID-TEN-002, BID-DOC-001 | Blocked | M2 |
| [BID-SEC-003](security/BID-SEC-003.md) | Independent assessment/remediation | E-SEC | P1 | M | BID-OPS-002, BID-QA-002, BID-DOC-002 | Blocked | M4 |
| [BID-SEC-004](security/BID-SEC-004.md) | Retention/legal-hold execution | E-SEC | P1 | M | BID-API-010, BID-AUTH-003, BID-OPS-003 | Blocked | M7 |
| [BID-QA-001](qa/BID-QA-001.md) | Multi-tenant real-dependency harness | E-QA | P0 | M | BID-TEN-002, BID-DB-003, BID-FND-003 | Blocked | M2 |
| [BID-QA-002](qa/BID-QA-002.md) | Complete external journey regressions | E-QA | P1 | M | BID-FE-008, BID-OPS-002, BID-QA-001 | Blocked | M4 |
| [BID-QA-003](qa/BID-QA-003.md) | Accessibility/mobile acceptance | E-QA | P1 | M | BID-FE-008, BID-ARC-006 | Blocked | M4 |
| [BID-QA-004](qa/BID-QA-004.md) | Capacity/concurrency/recovery load | E-QA | P1 | M | BID-OPS-002, BID-OBS-002, BID-DOC-003, BID-ARC-007 | Blocked | M4 |
| [BID-QA-005](qa/BID-QA-005.md) | Future procurement acceptance suites | E-QA | P2 | L | BID-BE-011, BID-BE-013, BID-ARC-009 | Blocked | M8 |
| [BID-QA-006](qa/BID-QA-006.md) | Complete Phase 1 portal acceptance | E-QA | P1 | M | BID-FE-009, BID-FE-011, BID-QA-001, BID-SEC-003, BID-OPS-003 | Blocked | M7 |
| [BID-OPS-001](devops/BID-OPS-001.md) | Reproducible CI/supply-chain gates | E-OPS | P0 | M | BID-FND-002, BID-API-001, BID-ARC-007 | Blocked | M1 |
| [BID-OPS-002](devops/BID-OPS-002.md) | Immutable isolated staging candidate | E-OPS | P1 | M | BID-OPS-001, BID-FE-008, BID-OBS-002, BID-SEC-002 | Blocked | M4 |
| [BID-OPS-003](devops/BID-OPS-003.md) | Restore/object/event reconciliation | E-OPS | P1 | M | BID-OPS-002, BID-TEN-002, BID-ARC-007 | Blocked | M4 |
| [BID-OBS-001](observability/BID-OBS-001.md) | Safe structured logging/correlation | E-OBS | P0 | S | BID-FND-001, BID-ARC-007 | Blocked | M1 |
| [BID-OBS-002](observability/BID-OBS-002.md) | Metrics/traces/alerts/runbooks | E-OBS | P1 | M | BID-OBS-001, BID-EVT-002, BID-TEN-002, BID-DOC-002 | Blocked | M4 |
| [BID-REL-001](release/BID-REL-001.md) | External-MVP evidence acceptance | E-REL | P1 | M | BID-ARC-001, BID-QA-002, BID-QA-003, BID-QA-004, BID-SEC-003, BID-OPS-003 | Blocked | M4 |
| [BID-REL-002](release/BID-REL-002.md) | Controlled pilot/feedback disposition | E-REL | P1 | M | BID-REL-001, BID-OPS-003 | Blocked | M5 |
| [BID-REL-003](release/BID-REL-003.md) | Human-authorized production promotion | E-REL | P1 | M | BID-REL-002, BID-OBS-002, BID-OPS-003, BID-SEC-003 | Blocked | M6 |
| [BID-REL-004](release/BID-REL-004.md) | Future capability-specific release gates | E-REL | P2 | M | BID-QA-005, BID-AI-004, BID-ARC-010 | Blocked | M8 |
| [BID-REL-005](release/BID-REL-005.md) | Phase 1 portal release acceptance | E-REL | P1 | M | BID-QA-006, BID-SEC-003, BID-OPS-003, BID-AI-005, BID-INT-004 | Blocked | M7 |

## Interpretation and maintenance

- Individual ticket metadata and this table must change together. `IMPLEMENTED_NOT_VERIFIED` records source-complete bounded functionality but does not satisfy acceptance criteria, dependencies or release gates. All prerequisites and approval blockers must close before Ready. A proposal ticket being accepted does not automatically approve the runtime work it describes.
- The Phase 1 gap audit created explicit shared-portal, platform-settings, commercial, PesaPal, support, retention, Tender Assistant, dashboard, web and acceptance children. Their presence is traceability, not implementation approval or completion.
- Remaining design packets for procurement and other advanced capabilities must create reviewed, granular runtime child tickets before implementation. Missing future child IDs are explicit blockers, not phantom completed dependencies.
- Features carry their own negative/unit/integration/UI tests; E-QA adds shared harness and release-wide evidence, not permission to postpone testing.
- API-008 owns the generated-client mechanism; later API tickets extend it with reviewed schemas before dependent UI. This does not imply future contracts are already generated.
- Use [delivery roadmap](04_DELIVERY_ROADMAP.md) for vertical slices and original BID-T000–017 mappings; use [graph](03_DEPENDENCY_GRAPH.md) for ordering.
