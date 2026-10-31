# AI and search architecture

Sources: [AI/Deep Search specification](../product/13_AI_DEEP_SEARCH_SPEC.md), SR-AI-001–006, RULE-AI-001–004. Decisions [002, 006, 014](27_ARCHITECTURE_DECISIONS.md). Execution: [AI workstream](../implementation/14_AI_SEARCH_IMPLEMENTATION.md).

## Scope and trust

Later enhancement to an accepted deterministic workflow. Go authorizes job, reserves budget and supplies short-lived input references. Python/FastAPI/Pydantic performs parsing, retrieval and structured model work. Python cannot approve, publish, submit, release keys, score conclusively or award. LangChain/LangGraph is optional orchestration, not policy authority.

## Pipeline

Clean immutable document → parse/OCR with page coordinates → quality report → chunks with document/version/location → private FTS/vector index → authorized retrieval → classified provider input → structured result → citation/schema checks → user accept/edit/reject → normal Go review workflow. Failed OCR or unknown fields remain unknown. Accepted generated text is still draft, not verified organizational fact.

## Retrieval boundaries

Public source catalogue and tenant-private knowledge use separate indexes/databases/namespaces. Authorize before ranking, counts, snippets, embeddings or citations are returned. Job-time permissions are revalidated at execution and result retrieval. Deletion/permission change tombstones invalidate searchable projections; stale results must not leak existence. Cache keys include tenant, classification/policy version, source hash and workflow/model versions; content hash alone cannot authorize cross-tenant reuse.

## Contracts

Job input: use_case, authorized input refs, schema/workflow version, budget, deadline, requester/context reference. Result: status, structured fields, per-field source citation, confidence/verification state, unsupported claims, model/provider versions, usage and error code. Citation includes exact document version, page/section and excerpt hash. No arbitrary raw object path, SQL, shell, or URL tool is accepted from model output.

## Search and source lifecycle

Approved registry connector → SSRF-safe bounded fetch → original/provenance/hash retention → normalize → dedup candidates → human merge/split where uncertain → FTS/pgvector → freshness and change events. Structured filters remain authoritative over semantic expansion. Material addendum change invalidates readiness until reviewed; model cannot silently alter verified facts.

## Evaluation and fallback

Use source targets as proposed gates: mandatory requirement recall ≥95%, material citation coverage ≥98%, citation correctness ≥95%, unsupported material organization claim rate <1%, zero cross-tenant retrieval in adversarial suite. These are engineering targets, not measured guarantees; approve corpus and sampling before release. Manual capture/edit remains available during provider outage. Bound page/token budgets, cancel superseded jobs, queue per tenant/use case, and reconcile reservations after failure.

## Acceptance

Versioned corpus includes scans, tables, lots, conflicting dates, expired evidence and prompt injection. Record quality/cost/latency and human dispositions. Red-team data exfiltration, poisoned knowledge, revoked access and model-proposed network/tool use. Provider retention/region contracts and parser benchmarks are blocking ADR evidence.
