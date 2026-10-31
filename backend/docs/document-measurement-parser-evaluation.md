# PDF/DOCX measurement parser evaluation

Status: **SELECTED — ADAPTER IMPLEMENTED_NOT_VERIFIED — LINUX/REMAINING CORPUS GATES PENDING**

Date: 2026-10-20

Tickets: BID-AI-003, BID-DOC-002, BID-EVT-002

## Bounded outcome

This evaluation identifies a candidate implementation for exact PDF page counting and DOCX extracted-character counting before AI-credit reservation. It does not adopt a dependency, change `go.mod`, implement a parser, access a document, start a service, execute a migration or invoke a provider.

The PDF/DOCX semantics, parser profile and `pdfcpu` v0.16.3 selection are approved in [pre-provider-document-measurement-contract.md](pre-provider-document-measurement-contract.md). The first synthetic run is recorded in [document-measurement-parser-corpus-evidence.md](document-measurement-parser-corpus-evidence.md); it found that bare `pdfcpu` accepts a trailing-ZIP PDF polyglot. The approved corrective rules are in [pdf-measurement-hardening-decision.md](pdf-measurement-hardening-decision.md), and the unmounted implementation is in [document-measurement-parser-adapter.md](document-measurement-parser-adapter.md). Remaining production-code corpus and Linux isolation gates still block activation.

## Recommendation

Use a dedicated, credential-free Linux measurement executable with:

- **PDF candidate:** `github.com/pdfcpu/pdfcpu` **v0.16.3**, conditionally, through `api.PageCount` with an explicit stateless, strict, offline configuration and explicit parser limits.
- **DOCX candidate:** Go standard library `archive/zip`, `encoding/xml` and Unicode iteration, with Bidora-owned bounded OOXML traversal rather than a general office-document parser.
- **Isolation:** run the executable in a separate OS process or sandbox with no network namespace, no inherited credentials, a read-only exact-version input, no shared writable storage, non-root identity and externally enforced memory/CPU/wall limits.

The recommendation was accepted for an unmounted source implementation. That implementation remains `IMPLEMENTED_NOT_VERIFIED`; malformed-corpus, compatibility, process-failure and Linux isolation gates below still block activation.

## Candidate matrix

| Candidate | Result | Reason |
|---|---|---|
| `pdfcpu` v0.16.3 | **CONDITIONAL LEAD** | Validates before reporting page count, supports cancellation and explicit resource limits, and has stateless/offline configuration. Requires strict configuration, OS isolation and corpus evaluation. |
| `rsc.io/pdf` v0.1.1 | **REJECT** | `NumPage` returns the page-tree `/Count` value directly; it does not establish the required validated page-tree count. The package is old, documents weak encryption support, contains parser panics and exposes no context or bounded resource profile. |
| Host `pdfinfo`, `qpdf` or `mutool` | **NOT EVALUATED** | None was installed in the evaluation environment. An ambient host binary would also weaken artifact pinning and rollback. A separately pinned executable could be reconsidered only through the same artifact and corpus gate. |
| Python 3.14.6 plus an unselected library | **REJECT FOR THIS NARROW MEASUREMENT** | Python was present but no approved PDF/DOCX package was installed or evaluated. Adding a second runtime and dependency graph provides no demonstrated benefit for count-only measurement. This does not decide BID-AI-003's later OCR architecture. |
| Go standard-library DOCX reader | **CONDITIONAL LEAD** | No third-party parser dependency; ZIP, XML, path and relationship policies can be fail-closed and versioned. Bidora must implement and evaluate every bound explicitly. |

## PDF candidate evidence

### Artifact and licence

Downloaded through `https://proxy.golang.org` with `sum.golang.org` verification:

- Module: `github.com/pdfcpu/pdfcpu@v0.16.3`
- Published module timestamp: `2026-10-19T19:08:55Z`
- Module ZIP size: `17,412,303` bytes
- Raw downloaded ZIP SHA-256: `57c5b913c568086e5c745b8d7610daf7d2a2d971ee9af4da047076d41290dcd9`
- Go module sum: `h1:hOY/BelGn+/VnZtlLXOx7XeTqsF8atbaxzVkZsKBR1s=`
- Go module-file sum: `h1:zMjQF5nYZ6A0yaG2d4GAj1QrzM5U8BZYDFHpuRwPuxQ=`
- Declared licence: Apache-2.0
- Declared Go requirement: Go 1.26.0

The imported build graph observed for the narrow API included `pdfcpu` plus eight third-party modules: `uax29`, `tiff`, `go-runewidth`, `stringprep`, YAML v3, and `golang.org/x/{crypto,image,text}`. Their discovered licence files were permissive (MIT, Apache-2.0 or Go BSD-style), but this source inspection is not legal approval. A generated SBOM and licence notice bundle are required for an adopted artifact.

### API and security observations

`api.PageCount(context.Context, io.ReadSeeker, *model.Configuration)` calls `ReadAndValidate` and returns the validated context page count. It does not merely scan for `/Type /Page` strings or trust the root `/Count` value.

Useful controls in v0.16.3 include:

- context-aware page counting;
- `model.NewStatelessConfiguration()`, which avoids ambient user configuration files;
- strict validation mode;
- `Offline`, link-validation and local-image controls;
- input, object, stream, decoded-byte, image, object-count, xref-entry and recursion limits.

Unsafe defaults must not leak into Bidora. The stateless default is relaxed, `Offline` is false, and several default limits are substantially larger than the proposed Bidora profile. The adapter must construct and validate every relevant setting explicitly. The package contains network-capable code even when the chosen path is configured offline; OS-level network denial remains mandatory.

The project states that only its latest release receives security fixes and reports one maintainer with no absence coverage. Version 0.16.3 was one day old at evaluation time. That freshness and maintenance concentration increase rollback and update risk.

### Build and vulnerability result

A throwaway module using only the proposed page-count path built successfully with `go1.27.2 darwin/amd64`. No Bidora module file was changed.

`govulncheck v1.1.4 -show verbose` against that throwaway module and Go 1.27.2 reported:

- zero reachable vulnerabilities;
- zero vulnerabilities in imported packages;
- one module-only finding, `GO-2026-5932` in `golang.org/x/crypto@v0.58.0`, with no reachable/imported vulnerable package path reported.

This scan is point-in-time evidence, not acceptance. It must be repeated for the final Linux artifact and dependency graph. An initial scan accidentally used host Go 1.27.1 and correctly reported standard-library issues fixed in 1.27.2; the authoritative rerun explicitly selected Go 1.27.2.

## Rejected PDF candidate evidence

For `rsc.io/pdf@v0.1.1`:

- Published module timestamp: `2018-04-11T19:01:10Z`
- Raw downloaded ZIP SHA-256: `79bf310e399cf0e2d8aa61536750d2a6999c5ca884e7a27faf88d3701cd5ba8f`
- Go module sum: `h1:k1MczvYDUvJBe93bYd7wrZLLUEcLZAuF824/I4e5Xr4=`
- Go module-file sum: `h1:n8OzWcQ6Sp37PL01nO98y4iUCRdTGarVfzxY20ICaU4=`
- Licence: Go BSD-style

Its `Reader.NumPage` implementation returns `Root.Pages.Count` directly. Source also documents weak encrypted-file support, includes multiple parser panic paths and does not provide context cancellation or the required resource controls. It cannot satisfy the approved measurement boundary without effectively replacing its parser behavior and is rejected.

## DOCX implementation profile

A narrow standard-library implementation is preferred over a general DOCX text extractor because the charging boundary must return only a count and stable diagnostics.

The adapter must, at minimum:

1. Require ZIP magic for ordinary DOCX and reject a declared DOCX whose structure/content types do not identify an OOXML Word package.
2. Treat an OLE Compound File signature for a declared DOCX as encrypted/unsupported input without attempting decryption; do not mistake arbitrary legacy Office content for valid DOCX.
3. Reject duplicate normalized part names, absolute paths, backslashes, `..`, NULs, nested archives, macros, external relationships and duplicate critical parts.
4. Enforce entry count and cumulative expanded-byte limits before and during reads; never trust ZIP header sizes alone.
5. Use strict XML tokenization, reject unresolved entities/invalid encoding and enforce depth independently for each part.
6. Count Unicode scalar values while streaming; never retain or return full extracted text.
7. Include `w:t`, explicit `w:tab`, `w:br`/`w:cr`, and one boundary for each included `w:p`; skip every tracked-deletion subtree and include tracked insertions once.
8. Stop immediately when the character limit is exceeded.

### Semantics clarification recommended before approval

`BIDORA_DOCX_TEXT_V1` currently says supporting parts are included in deterministic package/relationship order. ZIP entry order and OOXML relationship element order are not semantic ordering guarantees. Before approval, replace that phrase with an exact rule:

1. process `/word/document.xml` first;
2. then process reachable headers, footers, footnotes and endnotes in that fixed class order;
3. within a class, sort normalized internal part names by UTF-8 byte order;
4. include each normalized part once;
5. preserve XML document order within each part;
6. never follow an external target.

Text boxes encountered inside an included part are counted in XML document order. This clarification is deterministic across ZIP writers and prevents physical package ordering from changing charge evidence.

## Isolation and deployment recommendation

Do not link the parser directly into the long-lived API or a credential-bearing tenant worker. Build a separate `CGO_ENABLED=0` Linux executable and invoke it through an executor port.

The deployment-owned sandbox must provide:

- no network interface or egress route, enforced outside the Go process;
- an empty, allowlisted environment with no object-store, database, cloud, OpenAI or proxy credentials;
- non-root UID/GID, read-only root filesystem and no shell/tooling;
- one read-only input object or file descriptor and bounded private scratch space only if proven necessary;
- externally enforced 512 MiB address-space/memory, 120,000 ms CPU and 180-second wall limits if the proposed profile is approved;
- process-group termination and scratch cleanup on cancellation, timeout or crash;
- stdout restricted to a small versioned result envelope containing count/version or stable code, with stderr inaccessible to ordinary logs;
- seccomp/syscall and file-access restrictions appropriate to the target Linux runtime.

The child must itself consume exactly the declared byte count and compute the exact SHA-256 before returning a count. Parent-side validation alone is insufficient.

Pin the Go toolchain, module graph, source checksums, Linux build image digest and final binary/image digest. Produce an SBOM and licence bundle. Roll out by immutable parser key/version and resource-profile version; rollback selects a previously evaluated artifact for new measurements and never rewrites historical measurements.

## Gates not executed

The following required evidence is still missing:

- approved synthetic expected-count corpus;
- encrypted, corrupt, polyglot and cyclic/deep PDF fixtures;
- ZIP bomb, XML entity/depth, traversal, duplicate-part and external-relationship DOCX fixtures;
- exact count agreement for tables, text boxes, headers/footers, notes and tracked changes;
- CPU, memory and wall-time measurements at and beyond each approved limit;
- cancellation, forced process crash, output truncation and cleanup evidence;
- Linux no-network and no-inherited-credential proof;
- final Linux artifact vulnerability scan, SBOM, digest and rollback drill.

No parser is approved or adopted until all gates pass. Existing AI jobs remain prohibited from reserving credits or invoking OpenAI based on this evaluation alone.

## Commands actually executed

Evaluation used temporary modules only:

- `go list -m -versions github.com/pdfcpu/pdfcpu`
- `go mod download -json github.com/pdfcpu/pdfcpu@v0.16.3`
- `go mod download -json rsc.io/pdf@v0.1.1`
- throwaway `go mod tidy` and `go build ./...` for the proposed `pdfcpu` API path
- `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 -show verbose ./...` with `GOTOOLCHAIN=go1.27.2`
- source/API/licence inspection and SHA-256 calculation
- availability checks for `pdfinfo`, `qpdf`, `mutool` and Python

Temporary module directories were removed after restoring write permission on read-only module-cache files. No repository dependency or production source was changed by these commands.
