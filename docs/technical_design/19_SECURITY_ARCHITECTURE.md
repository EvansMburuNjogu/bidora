# Security architecture

Sources: [security specification](../product/14_SECURITY_PRIVACY_COMPLIANCE.md), SEC-001–009, BR-006/014/043/051. Decisions [003, 007, 010–017](27_ARCHITECTURE_DECISIONS.md). Execution: [security plan](../implementation/15_SECURITY_IMPLEMENTATION.md).

## Threat/control/evidence matrix

| Boundary / threat | Design control | Required negative evidence |
|---|---|---|
| Browser/session theft and CSRF | TLS, host-only secure HttpOnly cookies, rotation, CSRF/origin validation, bounded sessions | Fixation, stolen expired token, cross-origin write and recovery replay |
| Tenant routing/IDOR | Membership-derived context, separate credentials, scoped list/count/export queries | Foreign IDs, forged selector, stale context, restored assignment mix-up |
| Role escalation/insider support | Default deny, grantable scopes, last-admin lock, expiring approved support grant | Self-elevation, expired support access, revoked approver |
| Upload/parser abuse | Immutable quarantine version, type/hash/scan limits, sandbox, safe previews | Polyglot, archive bomb, parser timeout, object overwrite after scan |
| SQL/HTML/export injection | Parameterized SQL, allowlisted identifiers/sorts, sanitized rendering, formula-safe export | SQL metacharacters, rich text payload, spreadsheet formula |
| URL fetch/SSRF | Registry-approved connector, protocol/IP/DNS/redirect validation, network egress restrictions | Loopback/private/metadata address, redirect and DNS rebinding |
| Pricing/evidence disclosure | Classification checks before data serialization, package-wide authorization, private storage | Technical user pricing list/export/download, signed URL wrong version |
| Async confused deputy | Job-scoped references, execution-time authority, minimal events | Forged tenant payload, revoked user, stale service credentials |
| AI leakage/automation | Authorized retrieval, provider data policy, tool allowlist, human transition gate | Prompt injection, private citation, model-generated approve/submit |
| Supply chain/secrets | Pinned dependencies/images, SBOM/scans, secret manager, no runtime DDL | Leak scanning, vulnerable image gate, compromised dependency response rehearsal |

## Sealed bids (future, blocked)

Encryption-at-rest is not sealing. Require unique package DEK, KMS/HSM-backed wrapping, separated release authority, authoritative time/state/quorum/conflict/assurance checks, tamper-evident receipt and release log. Ordinary procurement runtime must have no standing pre-opening decryption path. Exact cryptographic algorithms, key hierarchy, rotation, recovery and emergency release require BID-ADR-010 approval and independent review. No speculative crypto implementation is authorized.

## Privacy and audit

Classify public/internal/confidential/pricing/personal/sealed/authentication data. Minimize telemetry and processor egress. Retention/legal holds, data-subject exports/erasure, region and subprocessors require privacy owner approval. Business audit is distinct from diagnostic logs; append-only app permissions do not defend against a DB superuser by themselves. Critical exports require tamper-evident manifests and independently controlled retention/storage where approved.

## Release policy

No cross-tenant or sealed-bid confidentiality defect is waivable. Other critical/high findings require remediation or explicit risk process consistent with source gates; absence of tests is not risk acceptance. Run threat-model review per significant boundary change, tenant isolation suite for every data path, independent assessment before customer pilot, and key-release review before sealed release. Document incident authority for token revoke, tenant suspend, connector disable and secret rotation without a full redeploy.
