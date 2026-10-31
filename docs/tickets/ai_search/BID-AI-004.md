# BID-AI-004 — Deliver grounded extraction suggestions with human disposition and cost controls

## Metadata
- Ticket ID: BID-AI-004
- Epic: E-AI
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-AI-002, BID-AI-003, BID-BE-003
- Blockers: Approved advisory job/API/UI packet and benchmark thresholds
- Milestone: M7

## 1. Objective
Offer cited tender facts, checklist items, evidence and response suggestions without changing authoritative bid facts automatically.
## 2. Business Context
AI can reduce document review effort while preserving human responsibility for completeness.
## 3. Scope
Budget reservation, tender overview and checklist suggestions, structured generation, schema/citation validation, unsupported-claim flags, natural tender-language presentation and accept/edit/reject/report actions through normal Go commands.
## 4. Out of Scope
Autonomous approval/submission/scoring, arbitrary tools, uncited organization claims or unapproved provider fallback. Unified conversation/planning/preflight/addendum orchestration is owned by BID-AI-005.
## 5. Technical Requirements
Authorized retrieval only; model/prompt/tool/schema versions; token/page/time budgets; incremental cost settlement; cancellation/retry; injection-resistant tool allowlist; accepted suggestions remain drafts until normal review.
## 6. Proposed Implementation Approach
Implement approved model port; validate outputs before persistence; compose citation/source UI using approved AI contracts; route human dispositions to audited domain commands; benchmark and adversarially test.
## 7. Dependencies
[Retrieval](BID-AI-002.md), [parsing](BID-AI-003.md), [deterministic compliance](../backend/BID-BE-003.md).
## 8. Acceptance Criteria
- Given fabricated/invalid citation, then material claim is rejected or explicitly unsupported.
- Given model instruction to submit/approve, then no such tool/action executes.
- Given quota exhausted/provider outage, then manual workflow still works and spend is bounded.
## 9. Testing Requirements
Frozen corpus regression, prompt injection, cross-tenant citations, provider timeout, budget races and human-review E2E.
## 10. Definition of Done
Accepted quality/cost/privacy evidence and visible human control; no blanket accuracy claims.
## 11. References
[RULE-AI-001–004](../../product/05_DOMAIN_MODEL_AND_BUSINESS_RULES.md), [PR-TP-005/010/011](../../product/03_PRD.md), [Tender Assistant](BID-AI-005.md), [AI design](../../technical_design/18_AI_SEARCH_ARCHITECTURE.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
AI-generated text acceptance does not independently verify company claims.
