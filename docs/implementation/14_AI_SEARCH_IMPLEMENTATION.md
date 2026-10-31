# AI and search implementation

## 1. Purpose
Add grounded assistance after deterministic bid execution works.

## 2. Scope
Authorized ingestion, FTS/pgvector, structured extraction, citations, human disposition and approved-source discovery. Later release, not required for proposed first external MVP.

## 3. Product specifications
[AI specification](../product/13_AI_DEEP_SEARCH_SPEC.md), SR-AI-001–006, RULE-AI-001–004; M21/M22.

## 4. Technical designs
[AI/search](../technical_design/18_AI_SEARCH_ARCHITECTURE.md), [integrations](../technical_design/16_INTEGRATION_ARCHITECTURE.md), [QA](../technical_design/24_QA_TEST_ARCHITECTURE.md).

## 5. Prerequisites
Secure clean documents, exact source versions, job-time authorization, quotas, real corpus and manual workflow fallback.

## 6. Required decisions
BID-ADR-006 release deferral; 014 model/OCR/source providers, region/retention and benchmark; budgets approved by commercial owner.

## 7. Dependencies
Documents/authz/events/search ownership precede AI; extraction proposals cannot replace verified facts automatically.

## 8. Target files
Future ai/app domain/use_cases/ports/adapters/workflows/evals; Go job API; tenant index migrations; AI result/citation UI.

## 9. Sequence
Build corpus; define Pydantic schemas and citations; authorize scoped input refs; parse/OCR quality; index exact versions; retrieval filters before ranking; model adapter with budget; schema/citation validation; user disposition; benchmark regression; controlled rollout.

## 10. Domain/security
No SQL/shell/email/submission/award tool. Untrusted documents are data, not instructions. Private/public indexes separated; provider egress policy applied before model call. Accepted text remains draft.

## 11. Database/API/frontend
Go owns job authorization and business state; Python returns structured advisory output. Store model/prompt/schema versions and usage; UI shows unsupported/unknown fields and exact source links.

## 12. Tests
Prompt injection, cross-tenant citations, revoked job, expired evidence, malformed output, provider outage, token quota, recall/precision/citation and cost regression.

## 13. Acceptance
Approved benchmark thresholds pass on frozen corpus; no unauthorized retrieval or autonomous material transition; manual workflow remains usable on outage.

## 14. Done
Evaluation report, provider/privacy review, cost monitoring and human-review UAT accepted.

## 15. Risks
Hallucinated qualification, stale retrieval, hash-only cross-tenant cache reuse and uncontrolled spend; enforce scope/version/budget and uncertainty labels.

## 16. Evidence
Corpus hash, workflow/model/config versions, metrics, adversarial cases and user disposition traces.

## 17. Exclusions
AI scaffold in current task, autonomous approval/scoring, unmeasured public quality claims, unapproved scraping.
