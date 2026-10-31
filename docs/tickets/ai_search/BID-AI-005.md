# BID-AI-005 — Deliver the natural-language Tender Assistant

## Metadata
- Ticket ID: BID-AI-005
- Epic: E-AI
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-AI-004, BID-API-012, BID-BE-015
- Blockers: Approved corpus/quality thresholds, plan budgets, language policy and provider deployment acceptance
- Milestone: M7

## 1. Objective
Deliver source-grounded tender understanding, planning, drafting and preflight in the tender's language without technical model jargon.
## 2. Business Context
Powerful understandable AI is Bidora's primary Tenderpreneur differentiator.
## 3. Scope
Overview, checklist, Bid/No-Bid advice, evidence matching, response help, work-plan suggestions, permission-aware conversation, review/preflight, addendum impact and outcome-learning orchestration with human disposition.
## 4. Out of Scope
Autonomous decisions/approval/submission, arbitrary web access, uncited material claims and procurement evaluation.
## 5. Technical Requirements
Exact clean sources, approved knowledge only, citation validation, tender terminology, natural uncertainty wording, structured internal states, stale invalidation, `store:false`, bounded spend/time and no cross-tenant retrieval.
## 6. Proposed Implementation Approach
Implement one typed operation at a time through the approved model/retrieval ports; validate before persistence; keep provider calls outside SQL transactions; route accepted changes through normal commands.
## 7. Dependencies
[Grounded suggestions](BID-AI-004.md), [contracts](../backend_contracts/BID-API-012.md), [approved knowledge](../backend/BID-BE-015.md).
## 8. Acceptance Criteria
- Given tender terminology/language, then output preserves it unless the user explicitly requests translation.
- Given unsupported organization claim, then AI states the gap and cannot add it as fact.
- Given source/addendum change, then affected prior assistance is marked for review.
## 9. Testing Requirements
Frozen multilingual corpus, citation recall/precision, injection, tenant leak, stale source, cost race, provider outage and human-disposition E2E.
## 10. Definition of Done
All approved assistant operations meet measured quality/privacy/cost thresholds and remain human-controlled.
## 11. References
[Phase 1 AI](../../../backend/docs/phase-1-portal-functional-design.md), [AI architecture](../../technical_design/18_AI_SEARCH_ARCHITECTURE.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Natural language must not overstate certainty or obscure source differences.
