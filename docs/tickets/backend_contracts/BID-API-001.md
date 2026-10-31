# BID-API-001 — Approve OpenAPI conventions, health and safe error contracts

## Metadata
- Ticket ID: BID-API-001
- Epic: E-API
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: S
- Dependencies: BID-ARC-002
- Blockers: Contract-owner approval
- Milestone: M1

## 1. Objective
Establish versioned machine-readable contract conventions before handlers.
## 2. Business Context
Consistent errors and concurrency semantics let bid users recover safely rather than repeat irreversible actions.
## 3. Scope
OpenAPI 3.1 structure, success/error envelopes, live/ready schemas, UUID/decimal/time conventions, pagination, request correlation, If-Match and idempotency policy.
## 4. Out of Scope
Business endpoint implementation or treating proposed 428/context selector as already approved.
## 5. Technical Requirements
Explicit nullable/required distinctions; stable field errors; safe 401/403/concealed-404/409/422/429/503 behavior. Approve numerical limits and unknown-property handling. Examples must contain synthetic values only.
## 6. Proposed Implementation Approach
Translate technical conventions to schemas; review compatibility rules; lint/validate examples; publish baseline and generator selection contract.
## 7. Dependencies
[Conventions](../architecture/BID-ARC-002.md).
## 8. Acceptance Criteria
- Given malformed example, then schema validation fails.
- Given health response, then no DB locator or dependency credential is permitted.
- Given breaking enum/request change, then compatibility policy identifies it as review-required.
## 9. Testing Requirements
Schema/example validation, negative response fixtures and contract compatibility cases.
## 10. Definition of Done
Approved schemas/conventions and reproducible lint evidence; no runtime handler claimed.
## 11. References
[API-001–008](../../product/04_SRD.md), [API design](../../technical_design/10_API_CONTRACT_DESIGN.md), [contract plan](../../implementation/07_BACKEND_CONTRACTS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
OpenAPI-generated types cannot replace domain authorization.

### Subsequent bounded implementation
The user authorized the local health slice. The [published OpenAPI](../../../backend/api/openapi/health.json), generated types, JSON Schema fixtures, live HTTP checks and generator-drift gate now exist; see [execution evidence](../12_HEALTH_SLICE_EVIDENCE.md). The earlier preparation record below is historical. Full business conventions and contract-owner acceptance remain outstanding, so the full ticket is not marked Done.

### Draft preparation evidence
Prepared [foundation API contract packet](../11_FOUNDATION_API_CONTRACT.md) with embedded OpenAPI 3.1 JSON, operational endpoint cards, safe errors, readiness/drain semantics, correlation, browser smoke states and required negative tests. Standard-library checks parsed all 3 JSON blocks, resolved all 27 internal references and checked expected paths/examples. Full OpenAPI lint, JSON Schema validation, compatibility and runtime tests were not run; validators are not installed and no dependencies were added. Status remains Blocked pending ARC-002 closure and contract-owner acceptance.
