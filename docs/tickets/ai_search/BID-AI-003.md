# BID-AI-003 — Parse clean documents with bounded OCR and exact source coordinates

## Metadata
- Ticket ID: BID-AI-003
- Epic: E-AI
- Type: Engineering
- Priority: P2
- Status: Blocked
- Complexity: M
- Dependencies: BID-AI-001, BID-DOC-002, BID-EVT-002
- Blockers: Approved parser/OCR adapter, limits and result schemas
- Milestone: M7

## 1. Objective
Produce source-linked text/table/page results without fabricating unreadable content.
## 2. Business Context
Scanned tender requirements must remain inspectable when extraction quality is uncertain.
## 3. Scope
Python/FastAPI/Pydantic authorized job boundary, approved native parser/OCR adapters, page checkpoints, coordinates/hash, quality flags and cancellation.
## 4. Out of Scope
Processing quarantined files, arbitrary parser network access, deciding compliance automatically.
## 5. Technical Requirements
Exact immutable input version; job-time authority; sandbox CPU/memory/page/time limits; encrypted/corrupt input explicit error; parser/schema version and pages expected/processed recorded; no confidential raw prompts/logs.
## 6. Proposed Implementation Approach
Implement ports and bounded workers; parse fixture corpus; checkpoint durable page results; validate Pydantic output/citation mapping; integrate safe progress/result queries from approved AI packet.
## 7. Dependencies
[AI design](BID-AI-001.md), [clean document boundary](../documents/BID-DOC-002.md), [jobs](../events_workers/BID-EVT-002.md).
## 8. Acceptance Criteria
- Given corrupt/low-quality page, then result reports failure/uncertainty rather than invented text.
- Given parser crash, then retry resumes safely with exact version.
- Given expired authority, then private input is not fetched.
## 9. Testing Requirements
Malformed/scanned/native/table corpus, sandbox exhaustion, page-coordinate/citation correctness, retry/cancel and tenant isolation tests.
## 10. Definition of Done
Approved parser quality/resource thresholds and actual evaluation reports; manual workflow unaffected on outage.
## 11. References
[AI ingestion](../../product/13_AI_DEEP_SEARCH_SPEC.md), [integration design](../../technical_design/16_INTEGRATION_ARCHITECTURE.md), [AI plan](../../implementation/14_AI_SEARCH_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
OCR confidence is not human verification or legal authority.

### Pre-provider measurement contract

`backend/docs/pre-provider-document-measurement-contract.md` and `backend/internal/opportunity/application/document_measurement.go` now define the bounded exact-version measurement port needed before Commercial AI-credit reservation. The contract requires a current-authorized immutable `CLEAN` PDF/DOCX version, exact byte-count/SHA-256 consumption, no-network isolated parsing, explicit finite resource limits, versioned parser/semantics facts and deterministic applicability evidence. It returns only PDF page count or DOCX extracted-character count and no document text. Status: **SEMANTICS/PROFILES APPROVED / ISOLATED PARSER ADAPTER IMPLEMENTED_NOT_VERIFIED / UNMOUNTED**. The accepted assessment in `backend/docs/document-measurement-parser-evaluation.md` selects `pdfcpu` v0.16.3 and a bounded Go standard-library OOXML reader. The partial synthetic run in `backend/docs/document-measurement-parser-corpus-evidence.md` confirmed basic counts, malformed/encrypted handling and selected boundaries, but found that bare strict `pdfcpu` accepts a trailing-ZIP PDF polyglot. The approved `backend/docs/pdf-measurement-hardening-decision.md` closes that gap with `BIDORA_PDF_PAGE_COUNT_V1` and explicit `BIDORA_PDFCPU_LIMITS_PILOT_V1`. `backend/docs/document-measurement-parser-adapter.md` records the separate count-only executable and process adapter. Tenderpreneur migration `00029`, immutable measurement persistence and the unmounted measurement → Commercial credit calculation → durable entitlement-request coordinator are recorded in `backend/docs/document-measurement-checkpoint.md`. Provider claiming requires `READY_FOR_PROVIDER`, the exact measurement/link and a `RESERVED` checkpoint. `backend/docs/document-entitlement-acknowledgement.md` records the unmounted atomic acknowledgement application: exact reserved evidence creates readiness, while stable denial makes the linked job terminal `FAILED`. `backend/docs/ai-entitlement-acknowledgement-dispatch.md` records the unmounted server-owned leased dispatcher with exact-generation registry resolution and replay-safe publication fencing. `backend/docs/document-ai-accounting.md` records the unmounted exact reservation recheck and durable `SETTLE_PENDING`/`RELEASE_PENDING` reconciliation through tenant-common migration `00007`; successful document analysis settles the deterministic measured credits, while authorization loss before provider access safely requests release. The user approved Option 1 in `backend/docs/openai-ambiguous-outcome-decision.md`: a possibly executed request with no trustworthy result settles deterministic measured credits, persists no suggestions, records `PROVIDER_OUTCOME_UNKNOWN` and never retries automatically; proven no-effect failures release. The OpenAI adapter now returns typed effect classifications and the processing transaction atomically records terminal failure plus the matching durable settlement/release request. The user approved Option 1 in `backend/docs/document-measurement-lease-retry-decision.md`. Tenderpreneur migration `00030` plus the application/pgx boundary now implement five-minute UUID-fenced non-renewable leases, immutable attempt/outcome evidence, at most three attempts, 30-second/120-second transient backoff, stale-worker commit rejection and deterministic terminal failure. Context cancellation leaves the lease for fenced recovery rather than deliberately reclaiming early. Production-code corpus/boundary/Linux isolation evidence, dispatcher/completion service-loop composition, stale-cell/poison-event policy and worker integration remain blocked; this does not complete BID-AI-003.

### Analysis initiation checkpoint

Completing an exact verified MinIO upload now creates one durable `TENDER_ANALYSIS` job for the immutable document version. Its initial state is `WAITING_FOR_CLEAN` with `DOCUMENT_NOT_CLEAN`; a uniqueness constraint prevents duplicate analysis jobs. This deliberately records user intent without permitting a parser/model to read quarantined bytes. The approved continuation adds exact-version ClamAV release and OpenAI Responses extraction. A tenant worker claims jobs with `SKIP LOCKED`, rechecks the requester's current verified ACTIVE membership and deny-first upload capability, and never holds SQL transactions while streaming bytes to ClamAV or OpenAI. Only `CLEAN` releases analysis; malware produces `REJECTED` and cancels it, while scanner failure leaves the version quarantined. OpenAI uses an explicit environment-supplied API key/model, exact HTTPS endpoint, `store:false`, bounded streaming input/response, no proxy/redirect, strict duplicate/unknown-field rejection and a versioned JSON schema. Results persist only as `PROPOSED_UNVERIFIED` suggestions with exact quote plus page/section location and confidence; no AI result becomes a requirement, decision or satisfied state. Human review is now implemented as an explicit separate transition: bounded cited suggestions can be accepted into a matching `BID` workspace or rejected with a mandatory reason; the original model text/citation remains immutable, and acceptance creates a distinct human-owned `OPEN` requirement with an exact document-version link. Live ClamAV/OpenAI execution, provider privacy acceptance and retry policy remain pending. Status: **IMPLEMENTED_NOT_VERIFIED**.
