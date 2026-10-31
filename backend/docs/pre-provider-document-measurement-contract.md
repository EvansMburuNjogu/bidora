# Pre-provider PDF/DOCX measurement contract

Status: **SEMANTICS/PROFILES APPROVED — ISOLATED PARSER ADAPTER IMPLEMENTED_NOT_VERIFIED — UNMOUNTED**

Tickets: BID-AI-003, BID-DOC-002, BID-EVT-002

Approval: on 2026-10-20 the requesting user approved `BIDORA_DOCX_TEXT_V1` with the canonical supporting-part order below and `BIDORA_DOCUMENT_MEASUREMENT_PILOT_V1`, then authorized the isolated parser corpus gate. This does not approve a parser artifact or waive Linux isolation/resource evidence.

Production contract: `internal/opportunity/application/document_measurement.go`

## Outcome

Before Bidora reserves AI credits or sends document bytes to OpenAI, it must obtain a deterministic measurement for the exact immutable `CLEAN` object version:

- PDF: exact page count, charged as `PDF_PAGES`.
- DOCX: exact extracted-character count under one approved, versioned extraction semantics, charged as `DOCX_EXTRACTED_CHARACTERS`.

Measurement is evidence for charging and execution gating. It is not document verification, a citation, OCR output, human approval or permission to invoke a model. Manual work remains available when parsing is unavailable.

The source contract is provider-neutral and intentionally has no parser implementation or default resource profile. It cannot make existing jobs runnable by itself.

## Required sequence

1. Recheck the requester’s current verified/enabled membership, deny-first operation capability and exact server-owned assignment generation.
2. Lock/read the immutable document-version facts and require `CLEAN`, supported media type, positive size no greater than 25 MiB, exact SHA-256 and immutable storage version.
3. Open only that exact object version. Never follow a caller URI, object key, redirect, relationship target or external reference.
4. Run the approved parser in an isolated executor without network, cloud credentials, database credentials, object-store credentials, shell/tool access or writable shared storage.
5. Enforce the approved wall-time, CPU, memory, page, extracted-character, ZIP-entry, expanded-byte, archive-depth and XML-depth limits. Missing enforcement fails closed.
6. The adapter must consume exactly the recorded byte count and reproduce the recorded SHA-256. Partial reads, trailing bytes or a different digest fail.
7. Persist the immutable measurement, parser/profile versions and applicability hash before calculating credits.
8. Calculate credits with the existing Commercial rule, create the tenant entitlement checkpoint, and wait for a durable control acknowledgement. OpenAI invocation remains prohibited before `RESERVED`.
9. Recheck current authority, assignment, document state and exact measurement applicability immediately before provider invocation.

No SQL transaction is held while object bytes are streamed or a parser runs.

## Typed input and output

`MeasurementRequest` contains only tenant/document-version identity, media type, `CLEAN` state, byte size and SHA-256. Storage locators remain behind the exact-object port and are never accepted from an event or browser.

`DocumentMeasurement` contains:

- exact tenant and document-version IDs;
- exact media type, byte size and SHA-256;
- parser key and parser version;
- extraction-semantics version;
- one allowed unit and positive measured value;
- deterministic applicability hash over those facts.

The measurement boundary returns no extracted text. Future cited parsing stores text/coordinates through a separately reviewed result schema, preventing charging code, logs or control-plane events from becoming a document-content channel.

## Fail-closed result codes

Adapters must map private diagnostics to one stable code and retain wrapped causes only in restricted process memory:

- `INPUT_HASH_MISMATCH`
- `INPUT_SIZE_MISMATCH`
- `UNSUPPORTED_OR_MISMATCHED_FORMAT`
- `ENCRYPTED_DOCUMENT`
- `CORRUPT_DOCUMENT`
- `PAGE_LIMIT_EXCEEDED`
- `CHARACTER_LIMIT_EXCEEDED`
- `ARCHIVE_LIMIT_EXCEEDED`
- `XML_LIMIT_EXCEEDED`
- `RESOURCE_LIMIT_EXCEEDED`
- `PARSER_TIMEOUT`
- `PARSER_UNAVAILABLE`

No code may include filenames, object keys, document text, relationship targets, parser stderr or stack traces. Failure does not mark the document unclean, reserve credits or invoke OpenAI. Approved Option 1 in [document-measurement-lease-retry-decision.md](document-measurement-lease-retry-decision.md) permits only UUID-fenced five-minute leases and at most three attempts with 30-second/120-second transient backoff; deterministic failures terminate immediately.

## Approved PDF semantics

The approved hardening rules and local boundary observations are in [pdf-measurement-hardening-decision.md](pdf-measurement-hardening-decision.md). The unmounted source implementation is recorded in [document-measurement-parser-adapter.md](document-measurement-parser-adapter.md).

`BIDORA_PDF_PAGE_COUNT_V1` contract:

- Count pages exposed by the validated PDF page tree after the parser has rejected corrupt or encrypted input.
- Do not estimate from `/Type /Page` byte patterns, xref counts, rendered images or provider output.
- Reject cyclic/deep page trees, malformed xref/object streams, decompression bombs and counts above the approved profile.
- Measurement performs no OCR, JavaScript, embedded-file extraction, font fetching, URI resolution or rendering.

The selected source parser is `pdfcpu` v0.16.3 with `BIDORA_PDFCPU_LIMITS_PILOT_V1` and a 2,000-page limit. Any parser upgrade requires a new version and repeat evaluation; a citation maximum of 10,000 does not raise the parser limit.

## Approved DOCX semantics

`BIDORA_DOCX_TEXT_V1` counting rule:

- Count Unicode scalar values, not UTF-8 bytes or UTF-16 code units.
- Preserve source Unicode without normalization.
- Include current visible text from the main document body and tables in package order.
- Also include headers, footers, footnotes, endnotes, text boxes and tracked insertions once. Process `/word/document.xml` first; then reachable headers, footers, footnotes and endnotes in that fixed class order; within a class sort normalized internal part names by UTF-8 byte order; include each normalized part once; preserve XML document order within each part.
- Represent an explicit tab, line break and paragraph boundary as one character each.
- Exclude package metadata, styles, settings, comments, tracked-deleted text, macros, custom XML, embedded objects and external relationships.
- Do not follow hyperlinks or external relationships.
- Reject malformed XML, unsupported encryption, nested archives, path traversal, duplicate critical package parts, excessive entries/expanded bytes/depth and counts above the approved profile.

An alternative body-only profile requires a different semantics version and would generally under-measure visible tender content.

## Approved numeric profile

Deployment must supply every field in `MeasurementLimits`; there are no defaults. Security/operations must approve finite values for:

- maximum PDF pages;
- maximum DOCX extracted characters;
- maximum ZIP entries and expanded bytes;
- maximum XML and archive depth;
- maximum memory bytes and CPU milliseconds;
- wall timeout.

The approved conservative `BIDORA_DOCUMENT_MEASUREMENT_PILOT_V1` profile is:

| Limit | Proposed value |
|---|---:|
| PDF pages | 2,000 |
| DOCX extracted characters | 5,000,000 |
| DOCX ZIP entries | 10,000 |
| Expanded DOCX bytes | 100 MiB |
| XML depth | 256 |
| Archive depth | 1 (the DOCX package only; nested archives rejected) |
| Address-space/memory ceiling | 512 MiB |
| CPU time | 120,000 ms |
| Wall timeout | 180 seconds |

The 25 MiB upload ceiling remains fixed and cannot be raised by the parser profile. These values are approved policy but are not active configuration. Configuration validation alone does not prove OS/container enforcement.

## Parser evaluation gate

The non-adoptive candidate assessment is recorded in [document-measurement-parser-evaluation.md](document-measurement-parser-evaluation.md). It conditionally recommends `pdfcpu` v0.16.3 for PDF and a bounded Go standard-library OOXML reader for DOCX, but does not approve either adapter; malformed-corpus, resource and Linux isolation evidence remains mandatory.

Before adopting a parser/runtime, record:

- exact artifact/package versions, licences, digests and vulnerability review;
- supported PDF/DOCX variants and explicit unsupported cases;
- behavior on encrypted, corrupt, polyglot, zip-bomb, XML-entity, path-traversal and external-relationship fixtures;
- exact count agreement against an approved synthetic corpus;
- memory/CPU/time measurements at every limit;
- cancellation and process-crash cleanup;
- proof of no network and no inherited credentials;
- Linux deployment compatibility and patch/rollback procedure.

BID-AI-003’s broader OCR, source-coordinate and quality-result work remains blocked after measurement. OCR must never be used merely to derive PDF page count.

## Deferred integration and verification

The unmounted source now includes the selected parser dependency/process adapter and the persisted measurement → credit calculation → durable entitlement-request orchestration recorded in [document-measurement-checkpoint.md](document-measurement-checkpoint.md). It still does not execute a parser, object read, migration, entitlement event or provider call.

Remaining work includes:

- acknowledgement-dispatch service-loop composition and approved stale-assignment/poison-event reconciliation;
- runtime composition of settlement/release reconciliation and approved ambiguous-provider handling;
- functional, malformed-corpus, resource, process-failure, isolation and provider tests;
- migration execution, runtime composition and deployment.

The typed port, isolated source adapter and checkpoint orchestration are `IMPLEMENTED_NOT_VERIFIED`; `BIDORA_DOCX_TEXT_V1`, `BIDORA_PDF_PAGE_COUNT_V1`, `BIDORA_PDFCPU_LIMITS_PILOT_V1` and `BIDORA_DOCUMENT_MEASUREMENT_PILOT_V1` are approved policy. Complete production-code corpus/resource/Linux isolation evidence and runtime composition remain pending, so no parser is active.
