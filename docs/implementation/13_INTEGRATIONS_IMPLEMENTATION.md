# Integrations implementation

## 1. Purpose
Connect approved providers without leaking domain authority or tenant data into SDKs.

## 2. Scope
Storage/email first; sources, payment, OCR/model and enterprise connectors only in approved later slices.

## 3. Product specifications
[Technology decisions](../product/21_TECHNOLOGY_DECISIONS.md), [API webhooks](../product/11_BACKEND_API_DESIGN.md), BR-030/034/043.

## 4. Technical designs
[Integration matrix](../technical_design/16_INTEGRATION_ARCHITECTURE.md), [security](../technical_design/19_SECURITY_ARCHITECTURE.md).

## 5. Prerequisites
Provider owner, contract/terms, data classification, credentials mechanism, documented rate/timeout/retry policy and sandbox fixtures.

## 6. Required decisions
BID-ADR-014 provider/region/retention; 015 billing authority/entitlements; source crawling requires lawful-source approval.

## 7. Dependencies
Typed port and durable operation ID before SDK; jobs/outbox before asynchronous notifications; verified payment reconciliation before entitlement activation.

## 8. Target files
Future owner-module adapters, provider fixture contracts, secret configuration references and runbooks; no shared SDK leakage into domain types.

## 9. Sequence
Define typed inputs/results and failure taxonomy; implement fake contract; add real adapter with context deadlines/egress controls; bounded retry/circuit break; webhook signature/dedup; reconciliation; sandbox tests; monitor and rollout by feature flag.

## 10. Domain/security
Authenticate raw webhook bytes before parse; allowlist egress and redirect/DNS behavior; minimize personal/pricing payloads; provider response never directly approves/submits/awards.

## 11. Database/API/frontend
Persist operation/provider ID and safe state, not indiscriminate raw envelopes. UI shows pending/degraded/retryable, not success from provider request initiation.

## 12. Tests
429/5xx, timeout before/after effect, malformed/forged callback, out-of-order events, duplicate payment/notice, secret rotation, SSRF private IP/redirect/rebinding.

## 13. Acceptance
Adapter satisfies same port tests as fake; ambiguous side effects reconcile without duplicate charge or false delivery.

## 14. Done
Contract, operational owner, recovery/runbook, privacy approval and sandbox evidence linked.

## 15. Risks
Unreviewed source terms, provider retention, retries duplicating effects; block rollout until contract and reconciliation evidence exist.

## 16. Evidence
Provider/API version, fixtures, redacted request metadata, rate/timeout configuration and incident drill.

## 17. Exclusions
Selecting a cloud/payment/model provider by example, unrestricted scraping and arbitrary model-driven fetch.
