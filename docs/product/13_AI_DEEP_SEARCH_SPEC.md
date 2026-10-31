# TenderOS — AI and Deep Search Specification

## 1. Purpose

AI and Deep Search are product accelerators built on top of reliable procurement and bid workflows. They must increase speed, understanding, and evidence quality without becoming an unreviewable decision maker.

## 2. AI principles

1. Source-grounded by default.
2. Tenant- and permission-aware retrieval.
3. Structured output before prose where a business object is required.
4. Human review for all material decisions and generated content.
5. Deterministic controls remain authoritative for deadlines, permissions, calculations, state, and mandatory gates.
6. Model/provider portability through adapters.
7. Quality, cost, latency, and user disposition are measured.
8. Tender documents and web content are untrusted input and cannot override system policy.

## 3. AI use cases

## 3.1 Tenderpreneur AI

- Tender extraction and document map
- Requirement and evaluation-criteria extraction
- Bid/No-Bid assistance
- Company capability and evidence matching
- Compliance gap analysis
- Proposal section drafting and rewriting
- Evidence recommendation
- Contradiction and unsupported-claim detection
- Addendum impact analysis
- Submission audit
- Win/loss and lessons analysis

## 3.2 Procurement AI

- Requisition-to-tender draft assistance
- Specification and requirement quality review
- Evaluation criteria and evidence recommendation
- Date/weight/threshold consistency checks
- Restrictive or ambiguous wording alerts
- Clarification clustering and draft response
- Submission evidence location and summarization
- Side-by-side bid comparison
- Pricing outlier and historical comparison support
- Draft evaluation and award reports

## 3.3 Platform AI

- Source connector anomaly summaries
- Support-ticket triage using non-sensitive metadata
- Usage/cost anomaly detection
- Security/risk alert summarization

Platform AI does not receive private tenant content unless explicit support access and policy permit it.

## 4. AI architecture

```text
Core Go API
  ├─ authorizes use case and inputs
  ├─ checks quota/budget
  ├─ creates AI job
  └─ supplies signed internal references
        │
        ▼
Python AI API / Worker
  ├─ loads authorized source content through service ports
  ├─ parses/OCRs/chunks/classifies
  ├─ retrieves relevant evidence
  ├─ executes versioned workflow
  ├─ validates structured output and citations
  └─ stores result, quality, latency, and cost metadata
        │
        ▼
Model / Embedding / OCR / Search adapters
```

The Python service must not accept a tenant ID from the browser and then query data independently. The core API issues a short-lived, job-scoped authorization token/reference.

## 5. Technology baseline

- Python and FastAPI
- Pydantic for versioned input/output schemas
- LangChain and/or LangGraph for orchestration where they reduce implementation risk
- Direct provider SDK adapters where simpler
- OpenAI as an initial model provider option, with a provider interface for alternatives
- PostgreSQL full-text search and pgvector initially
- Optional OpenSearch at scale
- OCR adapter supporting scanned PDFs/images
- Object storage for original and derived artifacts
- OpenTelemetry and model/job telemetry

LangChain is an orchestration implementation detail, not the domain model or source of business truth.

## 6. Document ingestion pipeline

```text
Upload/import
→ quarantine
→ malware/type/size validation
→ checksum and provenance
→ text extraction
→ OCR for image-only pages
→ page/section structure detection
→ language detection
→ document classification
→ chunking with page/section coordinates
→ embedding/indexing in correct security namespace
→ quality report
→ human verification where required
```

### Ingestion states

- `UPLOADED`
- `QUARANTINED`
- `REJECTED_SECURITY`
- `EXTRACTING`
- `OCR_REQUIRED`
- `INDEXING`
- `READY`
- `READY_WITH_WARNINGS`
- `FAILED_RETRYABLE`
- `FAILED_FINAL`

### Quality metadata

- Pages expected/processed
- Text coverage
- OCR confidence
- Tables/forms detected
- Missing/encrypted/corrupt pages
- Language(s)
- Parser/OCR versions
- Citation resolution quality

## 7. Source citation model

Material outputs use citations such as:

```json
{
  "claim_id": "REQ-17",
  "claim": "A bid security of KES 500,000 is required.",
  "source": {
    "document_id": "...",
    "document_version": 2,
    "page": 18,
    "section": "Tender Data Sheet",
    "bounding_box": [0.10, 0.22, 0.91, 0.31],
    "text_hash": "sha256:..."
  },
  "confidence": 0.96,
  "verification_status": "USER_VERIFIED"
}
```

Page/section references are preferred; line coordinates may be used for extracted text. Citation links must continue to resolve to the exact source version.

## 8. Structured tender extraction

Output schema includes:

- Tender identity and buyer
- Publication/closing/opening/clarification/site-visit dates and timezone
- Lots and locations
- Eligibility and mandatory requirements
- Technical requirements and criteria
- Financial response and pricing instructions
- Score scales, weights, thresholds, pass/fail rules
- Required forms/declarations
- Bid security and validity
- Personnel, experience, financial capacity, certifications
- Submission channel, envelopes, file formats/sizes/names
- Contract terms and high-risk clauses
- Addendum references
- Unknown/ambiguous/conflicting items

Each field includes source, confidence, and verification status. The model must return `unknown` rather than inventing absent values.

## 9. Bid/No-Bid assistance

The system combines deterministic and AI-derived factors:

```text
Eligibility gate
+ mandatory readiness
+ capability match
+ relevant experience
+ certification/licence match
+ staff availability
+ contract value and capacity
+ margin/commercial attractiveness
+ delivery/legal risk
+ competition and strategic fit
+ preparation effort and time remaining
```

Weights are tenant-configurable. AI may summarize evidence and suggest scores; the final assessment and decision are recorded by users.

## 10. Compliance matching

Matching compares tender requirements with:

- Approved organization profile
- Current verified document versions
- Staff and qualifications
- Past projects and references
- Certifications/licences
- Manufacturer authorizations
- Approved standard responses

Output statuses:

- `SATISFIED`
- `LIKELY_SATISFIED_REVIEW_REQUIRED`
- `PARTIAL`
- `MISSING`
- `EXPIRING_OR_INVALID`
- `CONFLICTING`
- `NOT_APPLICABLE_CANDIDATE`

AI does not directly set final requirement status; a permitted user verifies it.

## 11. Proposal generation

Generation inputs:

- Exact tender requirement and evaluation intent
- Approved organization facts and knowledge
- Selected evidence
- User instructions and required tone/format
- Page/word limits
- Prohibited unsupported claims

Output:

- Draft response
- Source references for organization claims
- Open questions/placeholders
- Unsupported-claim warnings
- Suggested evidence
- Compliance coverage notes

Generated content remains draft and requires normal review/approval.

## 12. Procurement tender quality review

Checks include:

- Missing scope/deliverable/acceptance detail
- Ambiguous or non-measurable wording
- Contradictory dates, sections, values, or requirements
- Duplicate requirements
- Criteria not linked to evidence
- Weights/thresholds not reconciling
- Potentially restrictive requirements
- Unreasonable timelines
- Requirements inconsistent with stated procurement method/category
- Missing submission/opening/evaluation rules

The report distinguishes deterministic validation from AI interpretation.

## 13. Evaluation assistance

Permitted AI behavior:

- Locate and quote/summarize relevant submission evidence within copyright/policy limits
- Map evidence to criterion
- Compare submissions using the same published criterion
- Flag missing, contradictory, or unclear evidence
- Identify pricing outliers and arithmetic review candidates
- Draft narrative from human scores/findings

Prohibited behavior:

- Final scoring without human evaluator entry/approval
- Introducing unpublished criteria
- Recommending award based on protected or irrelevant attributes
- Revealing one bidder's confidential content to another
- Accessing financial envelopes during prohibited stages
- Automatically disqualifying or awarding

## 14. Deep Search module

## 14.1 Source types

- Official procurement APIs and feeds
- Government and public-body tender portals
- County, parastatal, university, hospital, and agency websites
- NGO, donor, bank, insurance, and corporate procurement pages
- Approved tender marketplaces/aggregators
- Email feeds and organization subscriptions
- Platform-hosted tender catalogue

Every source is registered with ownership, access method, terms/policy notes, country, categories, frequency, rate limit, parser, authoritative status, and health.

## 14.2 Retrieval pipeline

```text
Schedule/API trigger
→ fetch with rate limit and conditional request
→ preserve raw response/document and metadata
→ parse and normalize
→ extract tender facts
→ canonicalize buyer/category/location/date
→ deduplicate against existing opportunities
→ assign source confidence
→ index full text/metadata/vector
→ detect changes
→ notify affected saved searches/watchlists/workspaces
```

## 14.3 Search query processing

A query may contain natural language and explicit filters.

The search planner extracts:

- Keywords/concepts
- Category/products/services
- Buyer/entity
- Geography
- Value/currency
- publication/closing window
- procurement method
- required certification/experience
- exclusions
- source types

Structured filters remain authoritative; semantic expansion must be explainable and user-adjustable.

## 14.4 Ranking

Suggested normalized score:

```text
0.30 semantic/full-text relevance
0.25 product/service capability fit
0.15 experience/reference fit
0.10 certification/licence fit
0.08 geography eligibility
0.05 value/capacity fit
0.04 deadline feasibility
0.03 source confidence/freshness
```

Weights are configurable and should not imply qualification certainty.

Result explanations include positive and negative factors:

- “Strong match: software implementation and CRM experience.”
- “Potential gap: requires ISO 27001; no current verified certificate found.”
- “Source last checked 2 hours ago; buyer website is authoritative.”

## 14.5 Deduplication

Use deterministic and probabilistic signals:

- Exact reference number + buyer
- Canonical URL/document hash
- Title/buyer/date similarity
- Document similarity
- Lot/source relationship

A canonical opportunity retains all source links and change histories. Human merge/split tools handle uncertain cases.

## 14.6 Change detection

Monitor:

- Closing-date extension
- Addendum/new document
- Clarification publication
- Requirement/document change
- Cancellation/withdrawal
- Source removal or access failure

Workspaces linked to a changed tender receive an impact-analysis job. Readiness may be invalidated until review.

## 15. Deep research

“Research this tender” may gather public information about:

- Procuring entity and official source
- Related notices and addenda
- Similar historical tenders and public awards
- Publicly available organizational context
- Relevant market or price references where lawful and reliable

Output sections are labeled:

- Tender document facts
- Official external facts
- Other public-source findings
- AI interpretation
- Unknown/unverified

## 16. Prompt-injection and tool safety

- Treat all tender/web/document text as untrusted content.
- System policy and tool permissions cannot be modified by document instructions.
- Retrieval content is delimited and labeled.
- Tools use allowlisted operations and job-scoped credentials.
- No arbitrary URL fetch from model output; source-fetch service applies SSRF controls and source policy.
- No arbitrary SQL, shell, file path, email, or submission action from model output.
- Sensitive data egress filters and classification policies apply before model calls.

## 17. AI quality evaluation

Maintain versioned benchmark datasets for:

- Tender field extraction
- Requirement recall/precision
- Criteria/weight/date accuracy
- Citation correctness and coverage
- Compliance matching
- Unsupported-claim rate
- Addendum impact detection
- Evaluation evidence retrieval
- Search relevance and deduplication
- Bias and protected-attribute leakage

Release gates are use-case-specific. Example initial targets to validate and refine:

- Mandatory-requirement recall ≥ 95% on approved benchmark set
- Material fact citation coverage ≥ 98%
- Citation correctness ≥ 95%
- Unsupported material organization claim rate < 1%
- No cross-tenant retrieval in adversarial test suite

These are engineering targets, not public guarantees until measured in production-representative data.

## 18. Cost and latency controls

- Per-plan AI quotas and monetary budgets
- Input page/token limits and user confirmation for large jobs
- Model routing by complexity and data classification
- Cache deterministic extraction/artifacts by content hash and workflow version
- Batch embeddings and incremental reindexing
- Cancel stale/superseded jobs
- Track cost by tenant, use case, document, model, and release
- Queue priorities for interactive, deadline-critical, and batch work

## 19. AI audit record

Record:

- Tenant, user, use case, and authorized input references
- Prompt/workflow/model/tool/schema versions
- Provider and region/configuration where relevant
- Start/end, latency, retries, token/usage/cost
- Structured output, citations, validation, and safety findings
- User accepted/edited/rejected disposition
- Downstream record/version created from accepted output

Raw sensitive prompts/responses require controlled retention and access; do not log them indiscriminately.

