# Events and workers implementation

## 1. Purpose
Make asynchronous operations durable, replay-safe and observable across tenant databases.

## 2. Scope
Outbox/inbox, NATS JetStream, worker leases, job states, retries, dead letters and external-effect reconciliation.

## 3. Product specifications
SR-EVT-001–005, [HLD messaging](../product/08_HLD.md), [LLD outbox](../product/09_LLD.md).

## 4. Technical designs
[Events](../technical_design/17_EVENT_MESSAGING_DESIGN.md), [integrations](../technical_design/16_INTEGRATION_ARCHITECTURE.md), [operations](../technical_design/23_OBSERVABILITY_OPERATIONS.md).

## 5. Prerequisites
Reviewed event schema, owning DB transaction, durable job contract and service identity ACLs.

## 6. Required decisions
BID-ADR-004 approved broker; 013 retention/replay windows and operational budgets; external effect semantics per adapter.

## 7. Dependencies
Domain mutation/audit/outbox first; consumer before exposing queued feature; job-time authorization before private processing.

## 8. Target files
Future shared/eventing, cmd/worker, owner handlers, event schemas and fault-injection integration tests.

## 9. Sequence
Implement atomic outbox; bounded dispatcher leases; durable publish ack; consumer inbox/effects transaction; explicit ack; retry/dead-letter; operator replay with audit; fair tenant scheduling; shutdown lease release; external reconciliation.

## 10. Domain/security
Event actor/tenant fields are not authority. Resolve trusted job and current membership. No raw confidential content in payload. At-least-once only; no exactly-once claim.

## 11. Database/API/frontend
Database job state is source for 202/polling. Broker status does not imply package completion. Redis leases are not business records; restart must recover from durable state.

## 12. Tests
Crash before/after commit/publish/ack; duplicate/out-of-order/schema-invalid events; provider timeout after effect; lost lease, revoked user and poisoned message.

## 13. Acceptance
No lost committed event; no repeated domain transition on replay; external ambiguity becomes visible reconciliation state.

## 14. Done
Retry/DLQ policy, schema compatibility, alert/runbook and failure-injection evidence.

## 15. Risks
Inbox expiry enables duplicate side effect; unbounded tenant polling exhausts connections; configure replay retention and bounded fair scheduling.

## 16. Evidence
Event/operation IDs, redacted traces, recovery durations, backlog metrics and replay audit.

## 17. Exclusions
NATS as authoritative business store, arbitrary event SQL, secret payloads and unlimited retry loops.
