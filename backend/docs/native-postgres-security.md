# Native PostgreSQL version review — BID-FND-004

> Historical, dated security evidence, not a current production certification. [Related tooling is archived](../../archive/local-fixture/README.md); cleanup does not authorize database startup or installation changes.

Status: **BLOCKED for live database integration on system 15.17**. The original investigation below was documentation-only. The user subsequently authorized an isolated **15.19** installation, which was built and version-checked without replacing system tools or starting a cluster; see [installation evidence](native-postgres-installation.md). Remaining runtime/TLS/security review gates are not waived.

## Observations and upstream evidence

The earlier native `local-check --tools` invocation confirmed that `postgres`, `pg_ctl`, `initdb` and `psql` under `/usr/local/bin` all report **15.17 (Homebrew)**. This is version-consistency evidence only. It does not prove package provenance, linked-library patch levels or security acceptance.

Official upstream pages retrieved successfully on **2026-10-17**, with individual advisory retrieval beginning at **21:54 UTC**:

- [Versioning policy and current releases](https://www.postgresql.org/support/versioning/): current PostgreSQL 15 minor release is **15.19**; the major branch is supported through **2027-11-11**. Upstream recommends the current minor release.
- [PostgreSQL 15 security index](https://www.postgresql.org/support/security/15/): identifies fixes published after 15.17, including server and client issues.

Representative individually checked advisories (not an exhaustive inventory):

| Advisory | Component / upstream description | PostgreSQL 15 fix | Published | Upstream CVSS |
|---|---|---|---|---|
| [CVE-2026-6473](https://www.postgresql.org/support/security/CVE-2026-6473/) | Server allocation integer wraparound may execute code as the database OS user | 15.18; versions before it affected | 2026-05-14 | 8.8 |
| [CVE-2026-6477](https://www.postgresql.org/support/security/CVE-2026-6477/) | libpq large-object functions can overwrite client stack memory; affects relevant psql/pg_dump operations | 15.18; versions before it affected | 2026-05-14 | 8.8 |
| [CVE-2026-16239](https://www.postgresql.org/support/security/CVE-2026-16239/) | Server cursor lifecycle type confusion may execute code as the database OS user | 15.19; versions before it affected | 2026-08-13 | 8.8 |

The upstream affected-version ranges include 15.17. Local reachability/exploitability and any vendor backports have not been assessed; no package evidence of such backports is available. Loopback binding and synthetic fixtures are not substitutes for patch review. Do not infer that merely invoking the earlier `--version` checks exercised these vulnerabilities.

## Consequence and proposed next operation

- Do not use the observed 15.17 installation for Bidora live database integration or claim it security-approved.
- **15.19 is a proposed same-major native candidate**, not an accepted installation/security baseline. The previously researched 18.6 remains a different-major candidate, not a required upgrade of personal databases.
- Request explicit approval before installing host tooling. Prefer a separate Bidora-owned installation prefix and explicit binary directory, with no replacement of `/usr/local/bin` links, Homebrew services or existing clusters.
- Before installation, verify macOS 12.7.6 / x86_64 compatibility, source/artifact provenance and license, required linked dependencies and the approved resource/storage budget. Include downloads, build artifacts, installed binaries, future cluster/WAL/log files and retained experiment artifacts; do not silently treat 10 GiB as table size alone.
- Do not auto-upgrade a data directory, adopt a discovered personal cluster, start a service, generate credentials or execute bootstrap SQL. Those remain separate isolated-cluster/configuration/role/resource review gates.
- After approved installation, rerun explicit version consistency checks against the new directory, review the resolved dependency/security evidence, then proceed only with reviewed fixture operations. Tool-version parity still does not certify database readiness or security.

The current `--tools` command intentionally reports only exact version parity. Current usage notes target the isolated 15.19 installation; earlier 15.17 parity evidence is historical, not a recommendation to deploy that version. No production code, API contract, business schema or web behavior changes are made by this investigation.

## Checks and limitations

- Official version/security index and three individual advisory pages retrieved over HTTPS; upstream text was inspected.
- No dependencies were installed, no vulnerability payloads were executed, and no database or frontend was started or queried.
- Documentation links/structure checked separately. Application tests were not rerun for this documentation-only increment; preceding passing tests remain historical evidence for unchanged code.
- Remaining acceptance: explicit installation authorization, native candidate compatibility/provenance/dependency/security review, FND-005 acceptance and isolated cluster/role/resource sign-off. This document does not self-approve any of them.
