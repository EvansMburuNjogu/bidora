# BID-REL-004 — Gate future AI and closed-loop releases on capability-specific evidence

## Metadata
- Ticket ID: BID-REL-004
- Epic: E-REL
- Type: Testing
- Priority: P2
- Status: Blocked
- Complexity: M
- Dependencies: BID-QA-005, BID-AI-004, BID-ARC-010
- Blockers: Selected future release scope, runtime child tickets and independent governance approvals
- Milestone: M8

## 1. Objective
Prevent external-MVP approval from being reused as authorization for materially different AI/marketplace risks.
## 2. Business Context
Sealed bids and advisory models require separate confidentiality, timing, quality and procurement accountability evidence.
## 3. Scope
Future capability release matrix, independent crypto/security review, full procurement UAT, AI corpus/privacy/cost evidence when included, enterprise/retention/billing extensions as scoped.
## 4. Out of Scope
Automatic bundling of all future features, claiming design packets are runtime completion.
## 5. Technical Requirements
Each chosen capability has schema/API/runtime/test child coverage; no pre-opening decrypt path; exact deadline policy; human scores/awards; model output citations and dispositions; provider/source legal approvals.
## 6. Proposed Implementation Approach
Select release; replace umbrella planning dependency with reviewed capability-specific child gates if splitting AI from marketplace; reconcile evidence; run independent assessments/UAT; authorize flags only after sign-off.
## 7. Dependencies
[Closed-loop suite](../qa/BID-QA-005.md), [AI advisory evidence](../ai_search/BID-AI-004.md), [future scope](../architecture/BID-ARC-010.md). Current combined gate is conservative; splitting requires explicit backlog change, not bypass.
## 8. Acceptance Criteria
- Given missing crypto/temporal approval, then sealed capability cannot activate.
- Given failed AI benchmark/privacy control, then AI release remains blocked while deterministic workflow remains available.
- Given unsupported future feature, then it stays excluded explicitly.
## 9. Testing Requirements
Actual independent security, procurement E2E/load/DR and scoped AI benchmark/UAT reports.
## 10. Definition of Done
Capability-specific human release approvals and complete evidence; no future completion assumed now.
## 11. References
[Roadmap R3–R8](../../product/16_PRODUCT_ROADMAP.md), [risks](../../technical_design/28_TECHNICAL_RISKS.md), [release readiness](../../implementation/19_RELEASE_READINESS.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
This umbrella must not force AI into external MVP or merge incompatible acceptance policies.
