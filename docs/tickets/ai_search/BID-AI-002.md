# BID-AI-002 — Build tenant-authorized full-text/vector retrieval with rebuildable indexes

## Metadata
- Ticket ID: BID-AI-002
- Epic: E-AI
- Type: Engineering
- Priority: P0
- Status: Blocked
- Complexity: M
- Dependencies: BID-AI-001, BID-DOC-002, BID-EVT-002, BID-BE-015
- Blockers: Accepted index/query contracts; pgvector/model version/licence/security/resource benchmark; approved source/knowledge corpus
- Milestone: M7

## 1. Objective
Retrieve relevant evidence without exposing unauthorized snippets, counts or citations.
## 2. Business Context
Bid teams need searchable current knowledge and traceable tender discovery, not cross-tenant similarity leaks.
## 3. Scope
Every eligible `CLEAN` content document, approved chunk/index migrations, tenant-database PostgreSQL FTS/pgvector, local multilingual embedding adapter, source-versus-approved-knowledge states, authorization-before-ranking, rebuild generations, deletion/permission tombstones and explainable filters/freshness.
## 4. Out of Scope
OpenSearch, shared vector service, arbitrary cross-tenant embedding cache, external embedding egress without separate approval and automatic qualification verdicts.
## 5. Technical Requirements
Exact source/version/location/text hash; parser/chunker/embedding/index generation; typed bounded filters; classification and source/approval eligibility before ranking/counts; current permission at query/result access; cache includes tenant/classification/policy/workflow versions; incremental idempotent indexing, tombstone, purge and gap recovery.
## 6. Proposed Implementation Approach
Implement reviewed owner schemas/ports in each tenant database; combine PostgreSQL FTS with a benchmarked local `sentence-transformers` multilingual model and pgvector; index eligible source and approved-knowledge corpora with distinct states; apply scope before ranking/counts; build tombstone/purge/rebuild jobs; validate fixtures and performance.
## 7. Dependencies
[Design](BID-AI-001.md), [documents](../documents/BID-DOC-002.md), [jobs](../events_workers/BID-EVT-002.md), [organization knowledge](../backend/BID-BE-015.md).
## 8. Acceptance Criteria
- Given revoked document permission, then query/citation cannot reveal its existence.
- Given rebuild interruption, then retry produces consistent authorized generation.
- Given stale source, then freshness is visible rather than implied current.
- Given every eligible clean document, then processing reaches Ready, Ready with warning or a visible bounded failure without granting approval.
- Given approved deletion/revocation, then retrieval denies immediately and physical purge/rebuild excludes the content.
## 9. Testing Requirements
Cross-tenant IDs/snippets/counts, multilingual relevance corpus, source/approved-state filters, local-model reproducibility, tombstone/purge/rebuild/restore replay and bounded resource/latency tests.
## 10. Definition of Done
Measured authorized retrieval and rebuild evidence against approved contracts.
## 11. References
[SR-AI-002/005/006](../../product/04_SRD.md), [AI/search](../../technical_design/18_AI_SEARCH_ARCHITECTURE.md), [AI plan](../../implementation/14_AI_SEARCH_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Shared content hash never substitutes for authorization. Recommended candidates are pgvector, `sentence-transformers` and BGE-M3; exact versions/digests and licence/security/resource acceptance remain mandatory before adoption.
