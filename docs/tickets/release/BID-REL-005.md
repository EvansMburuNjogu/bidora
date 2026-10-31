# BID-REL-005 — Accept the Phase 1 portal release

## Metadata
- Ticket ID: BID-REL-005
- Epic: E-REL
- Type: Documentation
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-QA-006, BID-SEC-003, BID-OPS-003, BID-AI-005, BID-INT-004
- Blockers: Technical evidence, local embedding supply-chain/benchmark acceptance, PesaPal/OpenAI/SMTP deployment acceptance, legal/privacy validation, UAT and accountable sign-off
- Milestone: M7

## 1. Objective
Make one evidence-based release decision for the selected Tenderpreneur and Platform Admin Phase 1 scope.
## 2. Business Context
Implemented screens or sandbox success must not be mistaken for a safe commercial release.
## 3. Scope
Trace contracts/schemas/builds; review QA/security/restore/accessibility/provider/local-model/privacy/legal evidence; conduct Tenderpreneur and Platform Admin UAT; record accepted limitations, rollout, rollback and formal disposition.
## 4. Out of Scope
Procurement lifecycle acceptance, production deployment by this ticket and waiver of tenant-isolation defects.
## 5. Technical Requirements
Exact artifact/schema/config/model/index provenance; no blocking defects; provider/secret/backup/alert/runbook readiness; plan/payment/privacy/retention/legal wording accepted; canary and recovery plan.
## 6. Proposed Implementation Approach
Assemble a signed evidence register from child tickets; run persona UAT on the candidate; disposition every failure/deviation; obtain named product/security/operations/commercial approvals.
## 7. Dependencies
[Phase 1 verification](../qa/BID-QA-006.md), [security](../security/BID-SEC-003.md), [restore](../devops/BID-OPS-003.md), [AI](../ai_search/BID-AI-005.md), [PesaPal](../integrations/BID-INT-004.md).
## 8. Acceptance Criteria
- Given any required failed/skipped gate, then release acceptance is denied.
- Given UAT, then all four Tenderpreneur and six Platform Admin roles demonstrate permitted journeys and denials.
- Given approval, then exact artifact, schema, providers, settings and residual risks are recorded.
## 9. Testing Requirements
Review executed QA-006 and provider evidence; run deployment smoke/canary/rollback and persona UAT without production private data.
## 10. Definition of Done
Named authorized humans sign the exact Phase 1 candidate as accepted or reject it with owned remediation; documentation alone cannot self-accept.
## 11. References
[Phase 1 design](../../../backend/docs/phase-1-portal-functional-design.md), [onboarding/context](../../../backend/docs/phase-1-tenderpreneur-onboarding-context.md), [privacy design](../../../backend/docs/phase-1-data-privacy-retention-deletion.md), [gap audit](../14_PHASE_1_PORTAL_GAP_AUDIT.md), [AGENTS](../../../AGENTS.md).
## 12. V10 release-gate assessment — RELEASE DENIED 2026-10-23

The V10 backend gate did not reach its exit target. Six full-pilot live compositions remain blocked by missing current document-processing/TLS NATS JetStream fixture configuration, and no single account-to-archive crash/restart journey was executed. Current-schema restore, load/resource, privacy/log, independent security, provider deployment, frontend/accessibility, UAT, canary and rollback evidence remain absent. BID-QA-006, BID-SEC-003, BID-OPS-003, BID-AI-005 and BID-INT-004 are still Blocked.

Under this ticket's acceptance criteria, the required skipped/failed gates deny release acceptance. No artifact, deployment or product release was accepted, and no accountable human sign-off was inferred.

## 13. Risks and Notes
Acceptance applies only to the recorded scope and versions; later setting/provider changes require their own rollout evidence.
