# BID-API-012 — Define organization knowledge and Tender Assistant contracts

## Metadata
- Ticket ID: BID-API-012
- Epic: E-API
- Type: Design
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-API-001, BID-AI-001, BID-ARC-010
- Blockers: Knowledge approval policy, AI operation limits, evaluation thresholds and plan budgets
- Milestone: M7

## 1. Objective
Define permission-aware contracts for approved organization knowledge and natural-language Tender Assistant operations.
## 2. Business Context
AI is useful only when users can understand its advice and trace it to tender and organization sources.
## 3. Scope
Organization details; versioned logo/letterhead/template draft-preview-activate; company-document processing and Ready-for-AI status; profile/knowledge create-version-review-approve; tender overview, checklist, Bid/No-Bid advice, evidence matching, response help, planning, conversation, preflight, addendum impact and outcome-learning job/result/disposition contracts.
## 4. Out of Scope
Autonomous decisions, arbitrary web tools, procurement scoring and uncited material claims.
## 5. Technical Requirements
Clean immutable inputs, source versus approved-knowledge distinction, FTS/vector processing status, exact branding/source versions, execution-time authority, exact citations, tender-language output, natural labels, unsupported/unknown representation, prompt/parser/chunker/embedding/model/schema versions, budgets, cancellation and human disposition.
## 6. Proposed Implementation Approach
Define separate typed operation schemas; pair user wording with precise internal states; freeze corpus fixtures and generated client types.
## 7. Dependencies
[Base API](BID-API-001.md), [AI policy](../ai_search/BID-AI-001.md), [Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md).
## 8. Acceptance Criteria
- Given a material claim, then a source or explicit unsupported status is required.
- Given changed source, then prior result applicability is visibly invalidated.
- Given another tenant's knowledge, then the contract exposes no existence signal.
## 9. Testing Requirements
Schema/citation validation, multilingual examples, permission negatives, injection fixtures and generated drift checks.
## 10. Definition of Done
Reviewed operation schemas, language rules, corpus oracle and approval; no model accuracy claim.
## 11. References
[PR-TP-001/002/005/010/011](../../product/03_PRD.md), [onboarding/context](../../../backend/docs/phase-1-tenderpreneur-onboarding-context.md), [Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Friendly wording must not hide uncertainty or weaken authoritative state distinctions.
