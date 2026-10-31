# Dependency graph and critical chains

## Authority

The complete adjacency list is the Dependencies column of [all 110 master rows](02_MASTER_BACKLOG.md), mirrored in each ticket. A dependency points from prerequisite to dependent. All referenced IDs exist; current graph is acyclic. Approval blockers and explicit implementation authorization are additional gates, not implicit Done nodes.

The diagrams below are deliberately selected critical chains, not a replacement for the complete adjacency list. Independent approved work may proceed in parallel; no calendar estimates are implied.

## Approved local-only child

[BID-FND-004](foundation/BID-FND-004.md) is the explicitly authorized local subset described in [packet 13](13_BACKEND_FOUNDATION_UNBLOCKING.md). Following the user's explicit split/start request, Docker-free [BID-FND-005](foundation/BID-FND-005.md) precedes FND-004; it was subsequently implemented manually and is now In Review, not accepted Done. FND-004 retains reviewed local pins/configuration/roles and Docker availability as explicit blockers. The full expanded graph now has 298 dependency edges, including the FND-005 → FND-004 edge, the Phase 1 portal children, ARC-010 → ARC-011 and the Platform Administrator lifecycle/remediation prerequisites. This child does not satisfy or remove the broader ARC-002/007/FND-003 gates and is not a substitute for their Done status.

## Foundation and protected tenancy

```mermaid
flowchart TD
  A[ARC-002 root/tool approval] --> C[API-001 conventions]
  A --> F[FND-003 config/services]
  C --> G[FND-001 Go health]
  F --> G
  G --> W[FND-002 real web smoke]
  G --> O[OBS-001 safe logs]
  D[DB-003 audit/outbox/inbox] --> B[BE-001 transaction primitives]
  O --> B
  B --> E[EVT-001 durable dispatch]
  E --> N[INT-001 notifications]
  N --> I[AUTH-001 verified identity]
  I --> M[AUTH-002 membership/context]
  M --> J[EVT-002 authorized jobs]
  M --> Q[TEN-003 quotas]
  J --> P[TEN-001 provisioning]
  Q --> P
  S[DOC-001 internal storage adapter] --> P
  P --> R[TEN-002 bounded router]
  R --> H[QA-001 two-tenant harness]
  H --> X[SEC-002 isolation gate]
  X --> U[FE-001 authenticated tenant shell]
  X --> V[BE-002 first bid API]
  X --> L[DOC-002 exposed document operations]
```

All abbreviated labels mean BID-* IDs. The full table additionally captures contract, schema, threat-model and root dependencies. There is no cycle through initial storage: DOC-001 is an **internal adapter**, not a public document handler. Provisioning is control-authorized before a tenant exists; TEN-002 handles business tenant routing afterward. EVT-001 is internal delivery infrastructure; EVT-002 adds current-actor authorization. Initial identity notices therefore do not require a provisioned business database.

SEC-002 tests existing router/storage/application boundaries, then every exposed endpoint extends it. It does not depend on the bid/document endpoints it gates. QA-001 is shared fixture infrastructure; earlier tickets still own their own unit/integration tests.

## External bid business path

```mermaid
flowchart LR
  K[API-003 and DB-004] --> B[BE-002 verified tender/bid]
  B --> C[BE-003 requirements/readiness]
  D[DOC-002 clean evidence] --> C
  C --> R[BE-004 technical/financial revisions]
  R --> A[BE-005 immutable candidate approval]
  A --> P[DOC-003 exact package job]
  P --> E[BE-006 external proof]
  E --> O[BE-007 outcome corrections]
  B --> F2[FE-002 bid UI]
  D --> F3[FE-003 vault UI]
  C --> F4[FE-004 matrix]
  F2 --> F4
  F3 --> F4
  R --> F5[FE-005 response editor]
  F4 --> F5
  A --> F6[FE-006 review]
  F5 --> F6
  E --> F7[FE-007 package/proof]
  F6 --> F7
  O --> F8[FE-008 outcome]
  F7 --> F8
```

API-005 precedes DB-006/response/review; API-006 precedes DB-007/package/proof/outcome. Their omission from this display does not relax those hard dependencies. All frontend operations consume the accepted generated client, not fabricated DTOs or mocked completion.

## Release path

```mermaid
flowchart TD
  F[FE-008 completed UI chain] --> S[OPS-002 staging]
  O[OBS-002 monitoring] --> S
  S --> Q[QA-002 full real journey]
  Q --> X[SEC-003 independent assessment]
  S --> R[OPS-003 measured restore]
  S --> L[QA-004 load/concurrency]
  F --> A[QA-003 accessibility]
  X --> M[REL-001 accepted external MVP]
  R --> M
  L --> M
  A --> M
  M --> P[REL-002 controlled pilot]
  P --> G[REL-003 authorized production]
```

Staging deployment is not release acceptance. M4 contains staging/verification/restore as prerequisites to scoped MVP acceptance; M5 is controlled pilot, M6 production authorization. Thus no later milestone must complete before an earlier one.

## Future design branch

ARC-010 → API-009/DB-008 and BE-008 requisition → BE-009 publication → BE-012 sealed acceptance → BE-013 opening → BE-010 evaluation → BE-011 award → QA-005 → REL-004. ARC-009 temporal/crypto review also gates API-009/DB-008/sealing. All BE-008–013 are **design** packets. QA-005/REL-004 retain explicit missing-runtime-child blockers; those children must become graph nodes before Ready.

ARC-010/ARC-008 → AI-001 contracts/privacy/corpus; DOC-002/EVT-002 plus BE-015 approved organization knowledge → AI-002 tenant-local FTS/vector retrieval; AI-001/DOC-002/EVT-002 → AI-003 parsing; AI-002/003 + BE-003 → AI-004 human-reviewed suggestions; API-012/BE-015/AI-004 → AI-005 Tender Assistant. REL-005 now owns the selected Phase 1 release gate; REL-004 retains future procurement/remaining capability-specific acceptance.

## Phase 1 Tenderpreneur and Platform Admin branch

```mermaid
flowchart TD
  ARC[ARC-010 Phase 1 packet] --> AP10[API-010 platform contracts]
  ARC --> AP11[API-011 commercial contracts]
  ARC --> AP12[API-012 AI/knowledge contracts]
  AP10 --> AU3[AUTH-003 portal/platform authority]
  AU3 --> BE14[BE-014 settings/countries]
  AU3 --> AU4[AUTH-004 support access]
  AP11 --> TEN4[TEN-004 plans/subscriptions]
  TEN4 --> INT4[INT-004 PesaPal]
  AP12 --> BE15[BE-015 approved knowledge/letterhead]
  BE15 --> BE17[BE-017 branded proposal exports]
  BE15 --> AI2[AI-002 local FTS/vector context]
  AI2 --> AI5[AI-005 Tender Assistant]
  BE14 --> BE16[BE-016 dashboards]
  AU4 --> BE16
  INT4 --> BE16
  AI5 --> FE9[FE-009 Tenderpreneur portal]
  BE17 --> FE9
  BE16 --> FE9
  BE14 --> FE10[FE-010 platform users/settings]
  FE10 --> FE11[FE-011 platform commercial/ops]
  BE16 --> FE11
  FE9 --> QA6[QA-006 Phase 1 acceptance]
  FE11 --> QA6
  QA6 --> REL5[REL-005 Phase 1 acceptance]
```

SEC-004 retention/legal-hold execution additionally depends on API-010, AUTH-003 and measured restore work; it gates FE-011. These children make selected scope traceable, but all remain Blocked until their decisions, contracts and explicit implementation authorization are satisfied.

## Scheduling and validation

1. Resolve scoped decisions before implementation contracts; do not wait for all future ADRs.
2. Review contracts and persistence design together, then freeze bounded packet before handlers/UI.
3. Start each vertical slice only when its own gates hold. Prioritize user-operable increments over completing whole technical epics.
4. Compute topological order from metadata, not numeric ticket suffix. For example TEN-003 precedes TEN-001 despite numbering.
5. Validate no dependency milestone exceeds dependent milestone; all current metadata satisfies this. Milestones are latest evidence targets, not compulsory batch starts.
6. Revalidate after splits or new runtime children. Readiness also requires the [standards](06_TICKET_STANDARDS.md).

References: [implementation dependency matrix](../implementation/23_DEPENDENCY_MATRIX.md), [BID-ADR-018](../technical_design/27_ARCHITECTURE_DECISIONS.md), [module ownership](../technical_design/09_BACKEND_MODULE_DESIGN.md).
