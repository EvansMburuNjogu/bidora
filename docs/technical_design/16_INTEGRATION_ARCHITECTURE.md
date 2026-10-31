# Integration architecture

Sources: [technology decisions](../product/21_TECHNOLOGY_DECISIONS.md), [AI specification](../product/13_AI_DEEP_SEARCH_SPEC.md), [API webhooks](../product/11_BACKEND_API_DESIGN.md). Decisions [004, 014–017](27_ARCHITECTURE_DECISIONS.md). Execution: [integrations](../implementation/13_INTEGRATIONS_IMPLEMENTATION.md).

## Adapter contract

Application ports accept typed operation IDs and authorized references, not provider SDK objects. Every outbound call has context deadline, bounded response size, egress allowlist, redacted telemetry and a retry classification. Timeout/retry/concurrency values are configuration validated against provider limits; numerical production values need benchmarks. Retry only demonstrably idempotent calls; ambiguous side effects enter reconciliation, not blind replay. Open circuit after a measured failure threshold, allow bounded half-open probes, expose degraded state and keep durable work pending.

| Integration / status | Port input → output; authentication and data | Failure/recovery, limits, tests and owner |
|---|---|---|
| S3 / approved abstraction, provider pending | Initiate/Complete/GetVersion: authorized object ref + constraints → version/checksum; workload identity or scoped credentials | Multipart expiry, version-bound reads, quota reservation, orphan reconcile; fake port + real MinIO contract tests; storage/operations owner |
| Email / provider pending | SendNotice: delivery ID, approved template/version, permitted recipient, minimal fields → provider message ID | No bid/pricing attachment by default; TLS, secret-managed API key; per-provider rate limit, dedup key, ambiguous timeout reconciliation; Mailpit + webhook fixtures; notification owner |
| External sources / later approval | FetchSource: registry ID, approved URL, conditional headers → bounded content/provenance | DNS/IP validation on every redirect/connect, private-range and metadata egress deny, HTTPS allowlist, source terms/rate policy; recorded fixtures, DNS rebinding tests; connector owner |
| Parsing/OCR / later provider approval | ParseDocument: clean exact version, page budget → structured pages/coordinates/quality | Sandboxed parser, no arbitrary network; encrypted files/low coverage reported, never fabricated text; page/CPU/time budgets, resumable page checkpoints; malformed corpus tests; document/AI owner |
| Model/embedding / later provider approval | Generate/Embed: scoped job, classified citations, schema/version, budget → structured result/usage | Provider retention/region agreement, API key/workload identity, egress minimization, token budget, circuit-break and no automatic high-risk action; deterministic fake + evaluation corpus; AI/privacy owner |
| Search / approved PostgreSQL baseline, later feature | Index: authorized version/provenance → index generation; Query: filters + authorization → ranked IDs | Private/public separation, deletion tombstones, incremental rebuild and freshness; query timeout/limits, corpus relevance and isolation tests; search owner |
| Payment / provider pending, later | CreatePayment/Reconcile: invoice/operation ID → provider reference/status | Signed raw-body webhook verification, replay window/dedup; entitlement changes only after backend reconciliation; ambiguous charge timeout queried not recharged; sandbox fixtures; billing/finance owner |
| NATS / approved | Publish: event schema + minimal references → broker ack; Consume → local inbox/effects | TLS/service identities, subject ACLs, bounded consumers, retry/dead-letter; outbox remains authority; broker outage/replay tests; platform owner |
| Redis / approved ephemeral | RateLimit/Lease/Cache: namespaced key + TTL → temporary result | TLS/auth, no business source of truth; outage policy fails safe for abuse controls; loss/restart tests; platform owner |
| Enterprise connectors / future | Explicit versioned domain contracts, not unrestricted DB access | OIDC/SCIM/ERP/e-signature choices need ADR, consent/scopes and reconciliation policy; partner contract fixtures; integration owner |

## Inbound boundary

Verify signature over raw bytes before parsing; bound body size, timestamp/replay window, deduplicate provider event ID, persist safe envelope reference and acknowledge only durable acceptance. Process asynchronously. Invalid signatures never create domain facts. Outbound webhooks require tenant-approved destinations with SSRF-safe registration, signed payloads, replay IDs and delivery history.

## Verification

For each adapter record provider version, secret rotation, rate policy, timeout budget, failure codes, recovery operator and data classifications. Test provider 429/5xx, timeout before/after effect, malformed payload, duplicate/out-of-order callback and secret rotation. No future provider is approved merely because its name appears in a source recommendation.
