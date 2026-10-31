# Isolated native PostgreSQL 15.19 installation — BID-FND-004

> Historical installation evidence. Cleanup preserved the host installation without running or changing it. [Related tooling is archived](../../archive/local-fixture/README.md), not an active application dependency.

**Status: VERIFIED_TECHNICALLY for source build, installation and tool-version parity only. Database integration and human acceptance remain pending.**

The requesting user explicitly approved the proposed isolated installation after the [15.17 security review](native-postgres-security.md). Execution occurred on **2026-10-17 UTC**. No existing cluster, system link, service configuration or web file/process was changed. No database was initialized, started or connected to.

## Location and use

Installed prefix:

```text
~/.local/share/bidora/postgresql/15.19/install
```

Use the explicit directory, not the unchanged system PATH installation:

```sh
cd backend
BIDORA_ENV=local BIDORA_PROFILE=postgres-local \
BIDORA_LOCAL_CPUS=2 BIDORA_LOCAL_MEMORY_MIB=2048 BIDORA_LOCAL_STORAGE_GIB=10 \
BIDORA_PG_ADDR=127.0.0.1:15432 \
BIDORA_PG_BIN_DIR="$HOME/.local/share/bidora/postgresql/15.19/install/bin" \
BIDORA_PG_VERSION=15.19 go run ./cmd/local-check --tools
```

Actual result: `Native PostgreSQL tools match 15.19; security acceptance, database connectivity, roles and resource enforcement NOT checked.` This command invokes only `--version` on the four selected tools. It does not use the supplied endpoint.

`/usr/local/bin/postgres --version` and `/usr/local/bin/psql --version` still report **15.17 (Homebrew)**. No Homebrew upgrade, relinking, profile/PATH modification, service start or existing-data migration was performed.

## Source, license and platform

- Source: <https://ftp.postgresql.org/pub/source/v15.19/postgresql-15.19.tar.bz2>
- Published checksum: <https://ftp.postgresql.org/pub/source/v15.19/postgresql-15.19.tar.bz2.sha256>
- Download: **23,452,545 bytes**.
- Verified SHA-256: `e1a64a87a46b825b88c082e4518161a47aab53c45694964f8ba1df28f7859f89`.
- Checksum and archive came from the official HTTPS origin. This is integrity/source-origin evidence, **not independent signature or reproducible-build verification**.
- Before extraction, archive members were checked to be regular files/directories under the expected source root, without absolute paths, traversal or links. Summed unpacked regular-file bytes: **149,985,961**, below the 512 MiB extraction guard.
- License: PostgreSQL license; source `COPYRIGHT` was read and copied into `install/COPYRIGHT`. SHA-256: `3d6af92ff8a4c2cdf69afb1cf44edea727922f5cd0cf8b5f72b11cdecac8fdfd`.
- Host: macOS **12.7.6**, x86_64, Apple clang **14.0.0** (`clang-1400.0.29.202`), GNU make **3.81**, existing Apple Command Line Tools SDK. No compiler/SDK installation occurred.

## Build choices and limits

Built from the official release archive using a minimal fixed child environment:

```text
PATH=/usr/bin:/bin:/usr/sbin:/sbin
LC_ALL=C
LANG=C
CC=/usr/bin/clang
CFLAGS=-O2
MACOSX_DEPLOYMENT_TARGET=12.7
```

Successful configure arguments:

```text
--prefix=<isolated-prefix>
--without-readline --without-zlib --without-icu
--without-lz4 --without-zstd
--without-gssapi --without-ldap --without-pam
```

Then `/usr/bin/make -j2` and `/usr/bin/make -j2 install`. SSL is disabled by the upstream default, verified in generated `pg_config.h` (`USE_OPENSSL` undefined). Other optional-language/framework defaults were not enabled.

**Deliberately limited fixture-tools build:** no TLS, readline, ICU or optional compression/authentication dependencies. This avoids linking unreviewed host OpenSSL/other optional installations. It is **not a production build**, does not support compressed pg_dump archives, and does not grant an exception to transport-security requirements. Before any local database use, explicitly review a strictly isolated loopback-only no-TLS exception or prepare a separately reviewed TLS-enabled build. No credentials or private data may be used under an inferred exception.

The first configure attempt rejected explicit `--without-ssl` with `argument required for --with-ssl option`. No compilation or installation occurred in that attempt. The unsupported flag was removed, leaving the documented upstream default; the failed log/receipt were preserved. The second configure/build/install phases all exited 0, taking approximately **40.2 / 171.0 / 4.2 seconds** respectively.

## Resource observations

The native build used two make workers, per-file 256 MiB size limits, 900-second phase deadlines, and sampled process-group RSS/directory-size guards. Guards were set to stop at 1.5 GiB sampled group RSS or 2 GiB installation-workspace allocation, leaving headroom below the approved 2 GiB RAM / 10 GiB storage budgets.

- Peak sampled build-process-group RSS: **182,208 KiB**, about **178 MiB**.
- Post-install workspace allocation: **264,264 KiB**, including downloaded source, extracted/build tree, logs and installation; subsequent small JSON inventory is additional.
- Installation alone: **30,052 KiB**, already included in workspace total (do not double-count).
- Conservatively counting the previously measured shared Colima cache (349,904 KiB) and stopped Bidora profile/disk metadata (224 KiB), retained experiment artifacts total approximately **600 MiB**. No old artifact was deleted to achieve this figure.

These are **sampled observations and build-worker limits, not hard aggregate native CPU/RAM/filesystem quotas**. They do not resolve the future database-runtime resource-policy gate. The pre-existing host compiler/SDK/system libraries were not newly provisioned.

## Actual verification and retained artifacts

All following checks executed successfully:

- Source SHA-256 equality, archive member/size checks, license inspection/preservation.
- Configure, two-worker source build and isolated installation.
- Installed `pg_config --version` and `--configure`.
- Explicit `local-check --tools` against the new prefix: `postgres`, `pg_ctl`, `initdb`, `psql` all match 15.19.
- `otool -L`/`file` inventory of **73 Mach-O executable/shared-library paths**, with SHA-256 hashes. Dependencies resolve only within the isolated prefix or to `/usr/lib/libSystem.B.dylib`. This is not an OS/library vulnerability scan or proof that macOS 12 is security-supported.
- Existing `/usr/local/bin` postgres/psql remain 15.17.

Host-local evidence under `~/.local/share/bidora/postgresql/15.19/`:

- `source.sha256`, downloaded archive and source tree.
- `build-local.py`: one-off guarded operational script, not a supported production installer. It refuses an existing install prefix; do not blindly rerun it.
- `phase-0.log`, `build-receipt.json`: first failed configure attempt.
- `attempt-2-phase-{0,1,2}.log`, `build-receipt-attempt-2.json`: successful build/install and resource samples.
- `binary-inventory.json`: installed binary hashes, formats and linked-library paths.

**Not executed:** upstream `make check` (starts a test cluster), database integration/role/persistence tests, host upgrades, web checks/startup, production deployment or signature validation. Application source was unchanged, so existing Go unit suites were not rerun for this installation; the real Go CLI toolchain check was run.

## Subsequent local exception approval

The user explicitly approved the loopback-only synthetic no-TLS/SCRAM exception and monitored native resource-budget approach on 2026-10-17. See the [fixture approval record](native-postgres-fixture-plan.md). This resolves those two policy choices, not credential/role/bootstrap SQL/logging review, detailed monitor implementation or permission to start the cluster. Earlier warnings above describe the installation boundary and must not be read as broader runtime acceptance.

## Remaining gates and recovery

The tools are present; FND-004 is not Done. Remaining work includes reviewed native monitoring details, isolated target/role/credential/bootstrap SQL/logging design, FND-005 acceptance, real database integration tests and accountable human review. The narrowly scoped no-TLS and monitored-budget policy choices are approved as recorded above. Fixes in upstream 15.19 address the cited PostgreSQL advisories; broader platform/dependency/security acceptance is not implied.

No service rollback or database restore is needed because none was started or changed. Preserve the isolated prefix and evidence for review. Future cleanup must be explicitly scoped to this newly owned directory; do not delete or stop system PostgreSQL, old Colima profiles or shared caches. No automatic cleanup/upgrade/reset was added.
