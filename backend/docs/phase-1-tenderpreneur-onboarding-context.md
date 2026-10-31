# Phase 1 Tenderpreneur organization onboarding and AI context

Status: **APPROVED PRODUCT DIRECTION — IMPLEMENTATION NOT STARTED**  
Scope: Tenderpreneur organization setup, reusable branding, clean-document indexing and AI-ready company knowledge

## 1. Outcome

A new Tenderpreneur organization should prepare its company details, evidence and document branding before serious bid preparation so the Tender Assistant can use trustworthy context immediately. Urgent tender capture remains available while setup is incomplete, but organization-grounded drafting and evidence matching disclose their limitations until approved knowledge is ready.

## 2. Journey

```text
Verify shared account
→ select Tenderpreneur account type
→ create organization basics
→ choose trial or paid plan
→ reconcile PesaPal payment when required
→ provision private workspace
→ configure company details and letterhead
→ upload company documents
→ scan and parse exact versions
→ review AI-proposed company facts
→ approve reusable knowledge
→ build private full-text/vector context
→ invite team
→ add first tender
```

Onboarding is resumable. Payment, provisioning, scanning, extraction and indexing states remain durable and honest. Retry resumes the same operation rather than creating duplicates.

## 3. Minimum activation information

Required before workspace activation:

- registered/display organization name;
- country;
- business type;
- primary industry/category;
- work contact;
- preferred language;
- confirmed currency, timezone and locale;
- active trial or reconciled paid subscription;
- one active Organization Admin.

Registration/tax identifiers, website, detailed address, previous projects, staff, certifications and standard content may be completed progressively unless a specific plan, country or tender operation lawfully requires them.

## 4. Company-document checklist

The setup experience asks for applicable documents in understandable groups:

- registration/incorporation and tax compliance;
- licences, memberships, certifications and manufacturer authorizations;
- company profile, services and capabilities;
- project records, completion certificates and references;
- staff profiles, CVs and professional qualifications;
- quality, security, safety, privacy and sustainability policies;
- approved standard response and methodology material.

Sensitive financial and personal documents receive narrower classifications and are not automatically available to every role or AI operation.

## 5. Letterhead and branding

Organization Admins can manage versioned:

- logo;
- registered and trading names;
- registration/tax identifiers where approved for display;
- postal/physical address;
- phone, email and website;
- slogan;
- header/footer layout;
- page-number style;
- brand colours;
- approved disclaimer/footer text;
- default proposal template.

Functions include draft, safe asset upload, preview with synthetic content, activate, supersede and rollback. Assets must pass type/size/malware controls and cannot contain scripts, macros, remote resources or unapproved executable content.

Every generated PDF/DOCX records the exact letterhead/template version and content hash. Later branding changes never rewrite historical proposal revisions or packages.

## 6. Company knowledge approval

After a clean document is parsed, AI may propose company facts. The Organization Admin or separately eligible reviewer can confirm, edit, reject, classify or restrict each proposal. Only human-approved facts and exact source versions become reusable authoritative company knowledge.

Approval does not make an expired document current. Revocation, expiry or replacement removes the item from new authoritative retrieval while preserving immutable historical proposal/evidence links.

## 7. Every eligible content document receives retrieval context

Every eligible uploaded content document is processed after it reaches `CLEAN`:

```text
CLEAN exact version
→ parse text and coordinates
→ detect language and document class
→ create bounded source chunks
→ build tenant-private full-text and vector entries
→ expose status as Still processing / Needs review / Ready for AI
```

“Every document” means every eligible content-bearing tenant document. Malware/rejected, unreadable/encrypted, unsupported, pure branding-image, payment-secret, authentication-secret and legally prohibited content is not embedded. Exact duplicate bytes still retain separate document/version provenance; an optimization may reuse computation only inside the same tenant and security scope.

Tender documents may be indexed as unverified source material for extraction and citation after `CLEAN`. Company documents may be indexed in a restricted source corpus after `CLEAN`, but cannot be presented as approved organization evidence until human approval. Retrieval always distinguishes source material from approved knowledge.

## 8. Recommended open-source vector implementation

Use the existing database-per-organization boundary rather than introducing a shared vector service:

- PostgreSQL full-text search for exact/lexical retrieval;
- `pgvector` in each private tenant database for semantic retrieval;
- the approved Python/FastAPI AI worker for parsing, chunking and embedding jobs;
- `sentence-transformers` as the embedding adapter;
- `BAAI/bge-m3` as the leading multilingual embedding-model candidate;
- hybrid lexical/vector ranking with deterministic filters before ranking.

Reasons:

- multilingual retrieval suits tender documents;
- local embedding avoids sending every company document to an external embedding provider;
- tenant databases provide physical organization separation;
- PostgreSQL remains source-adjacent and rebuildable;
- no separate shared vector database is required for Phase 1.

Exact versions, model files/digests, licences, supply-chain scan, memory/CPU profile, benchmark corpus and pgvector compatibility must be reviewed before adoption. BGE-M3 is a candidate, not a verified production pin in this document. If it fails those gates, another approved local multilingual model must implement the same adapter contract.

OpenAI remains the selected generation provider for approved Tender Assistant operations. Local embeddings do not authorize broader OpenAI data egress; generation still receives only the minimum currently authorized source excerpts and uses the approved privacy controls.

## 9. Vector record requirements

Each chunk/index record retains:

- tenant and security namespace;
- logical document and immutable version;
- knowledge-record/version when applicable;
- page/section/paragraph coordinates;
- exact text hash;
- language and document class;
- classification and allowed policy scope;
- source state and human-approval state;
- issue/expiry/applicability facts;
- parser/chunker/embedding model and index-generation versions;
- creation, supersession, revocation and tombstone state.

Embeddings are derived search data, never business truth or authorization evidence.

## 10. Retrieval controls

Before ranking, counts, snippets or citations:

1. validate current identity/session;
2. validate active tenant membership and context generation;
3. route through the server-owned tenant assignment;
4. apply classification, role and resource scope;
5. restrict to eligible source/approval states for the operation;
6. search only that tenant database;
7. revalidate before returning exact source or invoking the model.

No cross-tenant vector cache is permitted. A matching hash or similar vector never grants access.

## 11. Lifecycle and deletion

New approved versions create new retrieval entries and supersede prior applicability without rewriting history. Expiry stops current-evidence presentation. Revocation or approved deletion creates an immediate authorization/tombstone effect, invalidates caches and pending jobs, and schedules physical full-text/vector removal. Rebuilds are generation-based, idempotent and safe after interruption.

A restored database must reconcile current permissions, holds, source states and tombstones before it can serve search.

## 12. User-facing readiness

Do not expose “embeddings” as the primary user concept. Show:

- Ready for AI;
- Still processing;
- Needs your review;
- Document expired;
- Update required;
- Not available to your role.

The setup dashboard summarizes company profile, branding, certificates, projects, team qualifications, standard material and AI knowledge readiness.

## 13. Tender start policy

Before creating a Bid workspace, require active organization/subscription authority, basic organization description and confirmed services/capabilities. Letterhead must be configured or explicitly deferred. Missing projects/certifications/staff knowledge does not block an urgent bid, but the product must state that evidence matching and organization-grounded drafting are limited.

Whenever a company claim enters a proposal, its source must be clean, approved, applicable, permitted and linked to an exact version.

## 14. Ticket ownership

- BID-API-012: profile, letterhead, processing/index status and retrieval contracts.
- BID-DOC-002: clean exact company/tender document eligibility and indexing event.
- BID-BE-015: details, branding, letterhead and knowledge approval.
- BID-AI-002: tenant-private FTS/pgvector lifecycle.
- BID-AI-005: retrieval-aware Tender Assistant.
- BID-BE-004: exact letterhead/template snapshot during proposal generation.
- BID-FE-009: guided setup and Ready-for-AI experience.
- BID-SEC-004: deletion/hold effects across source and derived indexes.
- BID-QA-006: isolation, expiry, tombstone, rebuild and historical-branding evidence.
