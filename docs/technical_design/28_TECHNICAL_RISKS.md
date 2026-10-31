# Technical risks and blockers

Sources: [BRD risks](../product/02_BRD.md), [open decisions](../product/22_GLOSSARY_ASSUMPTIONS_DECISIONS.md). Governing [decision register](27_ARCHITECTURE_DECISIONS.md). Execution: [release readiness](../implementation/19_RELEASE_READINESS.md).

Severity below is design/release impact, not a claim of an exploitable vulnerability in a nonexistent application.

| Risk | Severity / owner | Mitigation and closure evidence |
|---|---|---|
| Conflicting MVP acceptance means external release cannot be signed consistently | High / product | ADR-006 release matrix, explicit exclusions, pilot acceptance signatures |
| Frontend path mismatch creates duplicate app | Medium / architecture | ADR-005 accepted path and reference update before scaffolding |
| Tenant routing/cache/projection leaks | High / security/backend | Context-race and multi-tenant API/storage/job tests, credential/pool isolation proof |
| Custom auth complexity underestimated | High / security | ADR-007 identity choice, threat model, recovery/MFA and independent review |
| Unapproved approval/waiver policy becomes hidden rule | High / product/governance | ADR-011 policy matrix, separation-of-duty and override tests |
| Missing deadline/acceptance semantics | High / procurement | ADR-009 exact instants/equality/outage/reservation rules; boundary tests |
| Cross-plane submission and key-release failure | High / security/architecture | ADR-010 durable attempt state machine, KMS prototype, independent review and fault injection |
| Upload parser exploit or scan/version TOCTOU | High / document/security | Exact version binding, sandbox/resource limits, approved format corpus |
| Tenant fleet migration/connection overload | High / operations | Bounded pool/migration benchmarks, canary pause and restore proof |
| Outbox dedup falsely treated as external exactly-once | High / backend | Provider operation IDs, effect state, ambiguous-timeout reconciliation tests |
| Pricing leaks via exports/notifications | High / security | Whole-package classification, field/recipient authorization tests |
| Undefined retention/provider region breaches obligations | High / privacy | ADR-013/014 processor and retention approval; deletion/hold and backup policy |
| Broad documentation mistaken for full future schema | Medium / delivery | Release-specific schema/API completion gates; explicitly track deferred procurement/AI/payment detail |
| Template unavailable/unlicensed | Medium / UX | Audited template or approved product-owned primitives; no unreviewed scripts |
| Speculative performance claims | Medium / operations | Workload profile, measured p95/p99 and recovery evidence |

## Escalation

Ticket names affected requirement, decision ID, owner and blocked operation. Stop only dependent implementation; do not bypass constraints to keep delivery moving. No response from an owner is not approval. Closure requires linked decision/test evidence, not changing status to green.

## Residual gaps

No approved cloud/CI host, production numerical budgets, retention schedules, package formats/canonicalization, parser/provider selection, complete future procedural schemas or full machine-readable API exists. These are explicit follow-up gates. This collection enables bounded engineering planning but does not certify production readiness or legal compliance.
