# PDF measurement hardening decision packet

Status: **APPROVED — ADAPTER IMPLEMENTED_NOT_VERIFIED**

Date: 2026-10-20

Tickets: BID-AI-003, BID-DOC-002, BID-EVT-002

Approval: on 2026-10-20 the requesting user approved `BIDORA_PDF_PAGE_COUNT_V1` and `BIDORA_PDFCPU_LIMITS_PILOT_V1` and authorized the isolated production measurement adapter. This does not authorize runtime mounting, document access, provider invocation or deployment acceptance.

## Approved decision

`BIDORA_PDF_PAGE_COUNT_V1` and `BIDORA_PDFCPU_LIMITS_PILOT_V1` close the observed trailing-payload and decompression-limit gaps and replace unsafe `pdfcpu` defaults with explicit limits.

## Approved `BIDORA_PDF_PAGE_COUNT_V1`

For an exact immutable `CLEAN` PDF version:

1. Consume exactly the recorded byte count and reproduce the recorded SHA-256.
2. Require `%PDF-` at the beginning accepted by strict `pdfcpu` validation; do not search arbitrary leading bytes for a PDF header.
3. Reject the input if Go `archive/zip.NewReader` recognizes the entire byte sequence as a ZIP archive with a prepended PDF payload.
4. Strip only PDF whitespace bytes NUL, HT, LF, FF, CR and SP from the physical end. The remaining bytes must end in `%%EOF`.
5. The final `startxref`, decimal offset and `%%EOF` must form one whitespace-separated final epilogue with no other bytes between them.
6. The final xref offset must be non-negative, before its `startxref`, and point to either `xref` or a syntactically recognizable indirect-object header for an xref stream.
7. If an earlier `%%EOF` exists, the final xref offset must be greater than the end of that earlier marker. This permits a real incremental revision and rejects a fake final epilogue that points back into an already-ended revision.
8. Run `pdfcpu` v0.16.3 with stateless configuration, strict validation, offline mode, link validation disabled, no local image root, `DecodeAllStreams=true`, and every limit below supplied explicitly.
9. Reject encrypted input without attempting a password. Reject malformed xrefs, page trees, object streams, stream filters and limit violations.
10. Count pages from the fully validated page tree. Reject zero pages or more than 2,000 pages.
11. Perform no OCR, JavaScript execution, embedded-file extraction, font fetching, URI resolution or rendering.

Whole-file ZIP recognition supplements rather than replaces strict PDF parsing. It avoids rejecting an ordinary PDF merely because an embedded stream happens to contain ZIP signature bytes.

## Approved `BIDORA_PDFCPU_LIMITS_PILOT_V1`

| `pdfcpu` setting | Approved value | Rationale |
|---|---:|---|
| `MaxInputBytes` | 25 MiB | Exact approved upload ceiling |
| `MaxObjectBytes` | 25 MiB | No single non-stream object may exceed the entire input ceiling |
| `MaxStreamBytes` | 25 MiB | No encoded stream may exceed the entire input ceiling |
| `MaxDecodeBytes` | 100 MiB | Bounds each decoded stream and combined page content below the 512 MiB process ceiling |
| `MaxImagePixels` | 25,000,000 | Bounds image-driven allocations even though page measurement does not render |
| `MaxImageBytes` | 100 MiB | Aligns decoded image allocation with the decode ceiling |
| `MaxObjectCount` | 250,000 | Bounds sparse object-number/xref allocation; substantially above the 2,000-page synthetic requirement |
| `MaxObjectStreamCount` | 50,000 | Bounds object-stream `/N` within the object ceiling |
| `MaxObjectStreamFirst` | 8 MiB | Bounds object-stream prolog allocation |
| `MaxXRefEntries` | 250,000 | Matches the object-number ceiling |
| `MaxRecursionDepth` | 100 | Retains `pdfcpu`'s finite default rather than increasing parser graph depth |
| `DecodeAllStreams` | `true` | Required so compressed bombs cannot bypass `MaxDecodeBytes` during page measurement |

The separately approved outer limits remain 2,000 pages, 512 MiB process memory, 120,000 ms CPU and 180 seconds wall time. The OS sandbox, not `pdfcpu`, enforces process memory/CPU/wall limits.

No field may be zero, negative or inherited from a user/configuration directory. A `pdfcpu` upgrade requires a new parser version and repeat evaluation; these values are not automatically portable to another parser.

## Local evaluation

A temporary Go 1.27.2 wrapper and synthetic corpus were removed after execution. No repository dependency or production parser was added.

### Tail and polyglot rule

| Fixture | Result |
|---|---|
| Ordinary strict PDF | Accepted |
| PDF followed only by PDF whitespace | Accepted |
| Valid incremental update with a new xref beyond the prior `%%EOF` | Accepted |
| Non-whitespace garbage after `%%EOF` | Rejected |
| Fake final epilogue pointing to the prior revision's xref | Rejected as non-monotonic |
| PDF with an appended valid ZIP archive | Rejected as whole-file ZIP polyglot |
| Appended ZIP plus fake PDF epilogue | Rejected as whole-file ZIP polyglot |

The first version of the tail check accepted a fake final epilogue. The evaluated second version added monotonic revision-offset and xref-target checks and rejected it. Signed PDFs, xref-stream incremental updates and linearized PDFs remain required compatibility fixtures.

### Internal limits

| Boundary | Observed |
|---|---|
| Trailer `/Size 250000`, maximum object number 249,999 | Accepted |
| Trailer `/Size 250001`, maximum object number 250,000 | Rejected by `MaxObjectCount=250000` |
| Nested object depth 99 | Accepted |
| Nested object depth 101 | Rejected at configured recursion limit 100 |
| Flate stream decoding to exactly 100 MiB with `DecodeAllStreams=true` | Accepted |
| Flate stream decoding to 100 MiB + 1 byte | Rejected by decode limit |
| Same 100 MiB + 1 stream with `DecodeAllStreams=false` | **Accepted**, proving `DecodeAllStreams=true` is mandatory |

The exact and over-limit decode cases each used a roughly 100 KiB compressed PDF. They completed in 0.83 and 0.26 seconds respectively and each observed approximately 278 MiB maximum resident set size on Darwin. This is below the proposed 512 MiB process ceiling but is not Linux acceptance or a concurrency capacity result.

The broader hardening corpus with tail, object and recursion cases completed in 0.67 seconds with approximately 8.6 MiB maximum resident set size, excluding the separate decoded-stream boundary runs.

### Dynamic evidence still missing

- exactly 25 MiB and 25 MiB + 1 input boundaries;
- object-buffer and encoded-stream boundaries;
- image-pixel and image-byte boundaries;
- exact recursion depth 100;
- xref entry and object-stream `/N`/`First` boundaries;
- valid linearized, signed and xref-stream incremental PDFs;
- malformed filter chains and dishonest stream lengths;
- timeout, forced crash and Linux process-limit enforcement;
- a representative synthetic compatibility corpus demonstrating that the conservative limits do not reject supported tender PDFs.

## Security disposition

The approved policy addresses the observed trailing ZIP and decompression-bomb bypasses. It does not make in-process parsing acceptable. The source adapter is implemented as a separate count-only process boundary; no-network, credential-free, non-root Linux execution with externally enforced limits remains deployment-owned and unverified.

Until the missing boundaries and Linux isolation are verified and the later checkpoint orchestration is implemented, no measurement checkpoint may become authoritative, no AI credit may be reserved from it and no OpenAI invocation may follow it.
