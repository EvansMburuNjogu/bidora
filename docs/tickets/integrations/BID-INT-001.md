# BID-INT-001 — Deliver scoped email notifications with duplicate-safe recovery

## Metadata
- Ticket ID: BID-INT-001
- Epic: E-INT
- Type: Engineering
- Priority: P1
- Status: Blocked
- Complexity: M
- Dependencies: BID-EVT-001, BID-ARC-008
- Blockers: Approved email adapter/data handling
- Milestone: M2

## 1. Objective
Reliably deliver verification, recovery and later workflow notices without private-data leakage.
## 2. Business Context
Teams cannot onboard or complete assigned reviews when notifications silently disappear or expose pricing.
## 3. Scope
Typed template/version/recipient port, control and tenant delivery identities, global authenticated-STARTTLS configuration by secret reference, synthetic test/activation/rollback, local fixture, retries/status and reconciliation.
## 4. Out of Scope
Marketing campaigns, raw bid attachments, plaintext credential storage, tenant-owned SMTP credentials and assuming email delivery means external bid submission.
## 5. Technical Requirements
Durable delivery ID/provider message ID; approved recipient resolution; minimal fields; challenge tokens handled through restricted channel without logging; verified authenticated STARTTLS; active tested configuration version; no secret readback; bounded retry/circuit break; ambiguous provider timeout reconciled rather than blindly resent.
## 6. Proposed Implementation Approach
Define delivery store/effect state with reviewed schema supplement; implement fake and real/local adapters; integrate outbox consumer; add provider callback verification if supported; monitor retry/failure.
## 7. Dependencies
[Outbox delivery](../events_workers/BID-EVT-001.md), [provider policy](../architecture/BID-ARC-008.md). Identity may use system-owned control notices before tenant jobs exist.
## 8. Acceptance Criteria
- Given duplicate event, then one logical notice is recorded.
- Given timeout after provider acceptance, then reconciliation preserves known identity.
- Given unapproved recipient/private content, then notice is rejected/redacted and audited.
## 9. Testing Requirements
SMTP/provider fixtures, STARTTLS downgrade/authentication, configuration rollback, 429/timeout/replay, redaction and recipient scope tests.
## 10. Definition of Done
Delivery/status/recovery evidence and runbook; no silent notification loss.
## 11. References
[PR-SH-006](../../product/03_PRD.md), [integration design](../../technical_design/16_INTEGRATION_ARCHITECTURE.md), [integration plan](../../implementation/13_INTEGRATIONS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Provider-specific idempotency is not universally available; expose uncertain delivery honestly.
