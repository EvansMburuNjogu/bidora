# Isolated document measurement parser adapter

Status: **IMPLEMENTED_NOT_VERIFIED — UNMOUNTED**

Date: 2026-10-20

Tickets: BID-AI-003, BID-DOC-002, BID-EVT-002

## Outcome

Bidora now has a count-only PDF/DOCX measurement executable and an outbound process adapter implementing `application.DocumentMeasurer`. It can produce deterministic measurement evidence for exact immutable bytes without returning extracted document text.

The executable is not mounted into a worker or API. The unmounted orchestration in [document-measurement-checkpoint.md](document-measurement-checkpoint.md) now defines exact object retrieval, immutable measurement persistence and durable entitlement-request creation, but no runtime invokes that source and no OpenAI invocation can follow it yet.

## Components

- `cmd/document-measurer/main.go`: strict framed stdin/count-only stdout executable.
- `internal/opportunity/measurementparser/`: approved PDF and DOCX parsing semantics.
- `internal/opportunity/measurementprofile/`: immutable parser, semantics and approved-profile identifiers and exact outer limits.
- `internal/opportunity/measurementprotocol/`: bounded private process protocol containing no tenant, user, object-storage or database identifier.
- `internal/opportunity/measureprocess/`: outbound adapter that launches the absolute executable directly without a shell or inherited environment.
- `internal/opportunity/application/document_measurement.go`: stable typed measurement failures with private wrapped causes.

`github.com/pdfcpu/pdfcpu@v0.16.3` is pinned in `backend/go.mod` and `backend/go.sum`. The parser dependency is reachable from `cmd/document-measurer`; static dependency inspection confirmed it is absent from the `measureprocess` parent adapter dependency graph.

## Process boundary

The parent writes:

1. a four-byte big-endian JSON-header length;
2. a strict JSON header no larger than 16 KiB;
3. the exact document bytes.

The child accepts only the approved profile, PDF/DOCX media types, positive input no greater than 25 MiB and a 32-byte SHA-256. It reads at most the declared size plus one byte, rejects short/trailing input, recomputes SHA-256 and emits one strict JSON result no larger than 16 KiB.

The result contains only success/failure, count, consumed byte/hash evidence and immutable parser/semantics/profile identifiers. It contains no text, filename, tenant/user ID, object key, relationship target, parser diagnostic or stack trace.

The process adapter:

- requires an absolute, clean, regular executable path and rejects symlinks at construction;
- invokes the executable directly without a shell;
- supplies an empty environment;
- uses `/` as its working directory;
- discards child stderr;
- bounds stdout;
- applies the approved wall timeout through `exec.CommandContext`;
- validates exact parser, semantics and profile identifiers before constructing `DocumentMeasurement`.

Deployment must still place this executable in a no-network, non-root Linux sandbox with read-only filesystem and externally enforced 512 MiB memory and 120-second CPU limits. Go process launch alone does not enforce those controls or verify an artifact digest against replacement after construction.

## PDF behavior

`BIDORA_PDF_PAGE_COUNT_V1` is implemented with:

- exact `%PDF-` prefix;
- whole-file ZIP-polyglot rejection;
- canonical final `startxref`/`%%EOF` epilogue;
- monotonic incremental-revision offset and xref target checks;
- stateless strict `pdfcpu` configuration;
- offline/no-link/no-local-image policy;
- `DecodeAllStreams=true`;
- every approved `BIDORA_PDFCPU_LIMITS_PILOT_V1` field supplied explicitly;
- encrypted/corrupt/resource/page-limit failure mapping;
- no OCR, rendering, external fetch or embedded-file extraction.

## DOCX behavior

`BIDORA_DOCX_TEXT_V1` is implemented with:

- ZIP entry, cumulative expansion and nested-archive bounds;
- normalized unique package part names and regular-file-only entries;
- exact non-macro DOCX main content type;
- strict XML with no directives/entities and explicit depth limits;
- rejection of external and traversal relationships;
- main document followed by header, footer, footnote and endnote classes, with UTF-8 byte-order sorting inside each class;
- exact WordprocessingML roots for included parts;
- Unicode scalar counting without normalization;
- tabs, breaks and paragraph boundaries counted once;
- tracked deletions excluded and insertions included;
- no returned or logged text.

## Failure behavior

Adapters expose only the approved stable codes through `application.MeasurementFailure`. Diagnostic causes remain unwrap-only for restricted internal handling. A parser crash, malformed protocol, oversized output or invalid child metadata fails as unavailable/mismatch rather than producing measurement evidence.

The executable cannot reserve credits or invoke OpenAI. A successful result is still non-authoritative until a later application use case rechecks authority/document applicability and persists the immutable checkpoint.

## Verification performed

Executed after implementation:

- `gofmt` over all changed Go files.
- `GOTOOLCHAIN=go1.27.2 go build ./...` — passed.
- Static dependency inspection — `pdfcpu` absent from `./internal/opportunity/measureprocess` and present under `./cmd/document-measurer`.
- Source/document integrity checks — passed.

No test body, parser corpus, subprocess execution, Linux sandbox, migration, database, service or provider was run in this implementation slice. Earlier temporary candidate corpus observations are not functional verification of this production code.

## Remaining work

- centralized deterministic parser/protocol/process tests under `backend/tests/`;
- approved compatibility fixtures for linearized, signed and xref-stream incremental PDFs;
- remaining exact internal PDF and dishonest ZIP boundaries;
- executable crash, timeout and partial-I/O tests;
- Linux no-network/no-credential and memory/CPU/wall enforcement evidence;
- final Linux binary/image digest, SBOM, licence bundle and vulnerability scan;
- runtime composition of the implemented acknowledgement/accounting boundaries and approved ambiguous-provider handling.
