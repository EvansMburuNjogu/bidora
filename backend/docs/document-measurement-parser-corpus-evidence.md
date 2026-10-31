# Document measurement parser corpus evidence

Status: **PARTIAL CANDIDATE EVALUATION — NOT ADOPTED**

Date: 2026-10-20

Tickets: BID-AI-003, BID-DOC-002, BID-EVT-002

## Scope

This evidence records a local synthetic evaluation of the conditionally selected PDF and DOCX measurement approaches. It used no tenant document, object store, database, service, migration or provider. All executables, modules and fixtures were generated in private temporary directories and removed afterward.

This is not the final parser gate. The run occurred on Darwin, used a temporary DOCX prototype rather than production code, and did not prove Linux resource enforcement, process-crash cleanup or every at-limit case.

## Environment

- Host: Darwin 21.6.0, x86_64
- Project-selected Go toolchain: Go 1.27.2
- PDF candidate: `github.com/pdfcpu/pdfcpu@v0.16.3`
- DOCX candidate: Go 1.27.2 standard library (`archive/zip`, `encoding/xml`, UTF-8 scalar iteration)
- Measurement profile: approved `BIDORA_DOCUMENT_MEASUREMENT_PILOT_V1`
- DOCX semantics: approved `BIDORA_DOCX_TEXT_V1` canonical part order

The host `go` command outside a toolchain-selected module reports Go 1.27.1. Every candidate build in this evidence explicitly selected Go 1.27.2.

## PDF results

The temporary adapter used `api.PageCount` with a stateless configuration, strict validation, offline mode, link validation disabled, no local image root and a 25 MiB input limit.

| Fixture | Expected | Observed | Result |
|---|---:|---:|---|
| Valid one-page PDF | 1 | 1 | Pass |
| Valid two-page PDF | 2 | 2 | Pass |
| Page tree with two children and `/Count 1` | Reject | Strict validation rejected counted/expected mismatch | Pass |
| Cyclic/self-referencing page tree | Reject | Strict validation rejected malformed parent relationship | Pass |
| Truncated PDF | Reject | Unexpected EOF | Pass |
| Non-PDF bytes | Reject | Missing PDF header/version | Pass |
| Password-encrypted PDF | Reject as encrypted | Password-required error | Pass; adapter still needs stable private error mapping |
| Valid PDF followed by ZIP EOCD bytes | Reject polyglot | **Accepted as one page** | **Blocking wrapper defect** |
| 2,000-page valid PDF | 2,000 | 2,000 | Pass at approved page limit |
| 2,001-page valid PDF | `PAGE_LIMIT_EXCEEDED` | Parser returned 2,001; temporary wrapper rejected | Pass at wrapper boundary |
| Pre-cancelled context | Cancellation | `context canceled` | Pass |

The seven-input small corpus completed in 0.01 seconds elapsed with 8,003,584 bytes maximum resident set size. The 2,000/2,001-page boundary run completed in 0.74 seconds elapsed with 26,320,896 bytes maximum resident set size. These are local observations, not ceilings or performance acceptance.

### PDF fixture SHA-256

- one page: `060e6f961e368475342fcc8915f0448e22e49b2532e3437dea82f946f3984310`
- two pages: `b859da9fead7725aea74699b1d70306e3b77aeae71f8df73670aff28cc9816e9`
- page-count mismatch: `ba3dfcb996fa9940bb07b56967ede5669c3ca7ec974083895509ce473ec42dc5`
- cyclic tree: `2c8bd7d603b3b3824573b14c0c3e8c298cf17ce614467032095ee6cfd4ef1fb2`
- encrypted: `cdf03ec2176948e5d5fce523ed06636484b7d42ffe4a794de5804549131e8332`
- corrupt: `34e2b5a1da2d54162a0a328a422951f5e4f4380a1edea9f2bf5d81be7bb29d48`
- non-PDF: `07bcbca5e5cff5eadb6a3578850bb9413f9bfd2406d9210b0c25b7f78449889a`
- trailing-ZIP polyglot: `57a6d54c6bb9dc6060acc8d835a25510ae290c106037908234f25d03ea96b48f`
- 2,000 pages: `90ea006fe901522dab9f377a7d5bc4e048bcf582264da06f8174d190b190ab57`
- 2,001 pages: `8ed4fc598664e65a0ebabbc2ece71ece4d8d55ee63bd6403c44b18a4a88a5d5d`

The corpus generator and fixtures were temporary. They must be persisted centrally before these observations can become reproducible acceptance evidence.

## PDF finding requiring correction

`pdfcpu` strict validation accepted a valid PDF followed by a ZIP end-of-central-directory record. The corrective `BIDORA_PDF_PAGE_COUNT_V1` tail/polyglot rule and explicit `BIDORA_PDFCPU_LIMITS_PILOT_V1` values are approved and locally evaluated in [pdf-measurement-hardening-decision.md](pdf-measurement-hardening-decision.md). The unmounted adapter implements them, but activation remains blocked by missing production-code compatibility, boundary and Linux evidence; relying on library defaults is prohibited.

## DOCX results

The temporary prototype enforced the approved ZIP-entry, expanded-byte, XML-depth and character limits; duplicate/path checks; no external relationship traversal; canonical supporting-part order; strict XML; tracked-deletion exclusion; and Unicode scalar counting.

| Fixture | Expected | Observed | Result |
|---|---:|---:|---|
| Body Unicode/tab/break/paragraph corpus | 9 | 9 | Pass |
| Tracked deletion plus insertion | 4 | 4 | Pass |
| Table and nested text box | 10 | 10 | Pass |
| Main + header + footer canonical order | 15 | 15 | Pass |
| 5,000,001 text scalars | Character limit | `CHARACTER_LIMIT_EXCEEDED` | Pass beyond limit |
| XML depth 257 | XML limit | `XML_LIMIT_EXCEEDED` | Pass |
| Duplicate critical part | Archive limit | `ARCHIVE_LIMIT_EXCEEDED` | Pass |
| More than 10,000 ZIP entries | Archive limit | `ARCHIVE_LIMIT_EXCEEDED` | Pass |
| More than 100 MiB declared expansion | Archive limit | `ARCHIVE_LIMIT_EXCEEDED` before expansion | Pass |
| External relationship | Reject | `ARCHIVE_LIMIT_EXCEEDED` | Pass |
| Relationship traversal | Reject | `ARCHIVE_LIMIT_EXCEEDED` | Pass |
| ZIP entry traversal | Reject | `ARCHIVE_LIMIT_EXCEEDED` | Pass |
| Nested ZIP | Reject | `ARCHIVE_LIMIT_EXCEEDED` | Pass |
| XML entity/DOCTYPE | Reject | `XML_LIMIT_EXCEEDED` | Pass |
| Malformed XML | Reject | `CORRUPT_DOCUMENT` | Pass |
| OLE compound-file signature | Encrypted/unsupported | `ENCRYPTED_DOCUMENT` | Pass as conservative classification |

The complete 16-input DOCX run completed in 0.95 seconds elapsed with 28,741,632 bytes maximum resident set size. The expanded-byte bomb was rejected from ZIP metadata before decompression. This does not prove behavior for dishonest ZIP metadata, data descriptors, CRC failures or interrupted reads.

The DOCX fixture hashes were captured in command output, but the generator and fixtures were temporary. They are not treated as durable evidence. The next implementation must place deterministic synthetic generators and external-package fixtures under `backend/tests/` without document text entering ordinary logs.

## Local isolation probe

A Darwin `sandbox-exec` profile with `deny network*` was launched through an empty environment containing only:

- `PATH=/usr/local/bin:/usr/bin:/bin`
- `HOME=/nonexistent`
- `TMPDIR=/tmp`

A TCP connection probe failed with `PermissionError`, errno 1 (`Operation not permitted`). This demonstrates the local profile's network denial and sanitized launch environment, but it is not Linux deployment proof and did not exercise a production parser executable.

## Gate disposition

The local candidate results support continuing with `pdfcpu` v0.16.3 and the standard-library DOCX approach, but adoption remains blocked by:

1. Remaining production-code PDF compatibility and exact internal-boundary evaluation recorded in the hardening packet.
2. Centralized protocol/parser/process tests for exact byte-count/SHA-256 consumption and cancellation.
3. Persisted checkpoint and application orchestration without holding a SQL transaction during parsing.
4. Deterministic centralized fixture generators and expected-result manifests.
5. Exact-at-limit DOCX character, ZIP-entry and expanded-byte cases.
6. Dishonest ZIP metadata, CRC, data-descriptor and partial-read cases.
7. Forced timeout/crash/process-group cleanup evidence.
8. Linux memory/CPU/wall enforcement and no-network/no-credential proof.
9. Final Linux artifact SBOM, digest and vulnerability scan.

No AI credit may be reserved and no OpenAI call may occur based on this partial evidence.
