# BID-INT-002 — Implement one approved external-source connector with provenance and safe fetch

## Metadata
- Ticket ID: BID-INT-002
- Epic: E-INT
- Type: Engineering
- Priority: P2
- Status: Blocked
- Complexity: M
- Dependencies: BID-ARC-010, BID-EVT-002, BID-DOC-002
- Blockers: BID-ADR-014 source terms/provider and reviewed connector schema/contracts
- Milestone: M7

## 1. Objective
Import one lawful source through a bounded reproducible adapter, retaining original provenance.
## 2. Business Context
Discovery is useful only if bidders can inspect source freshness and distinguish verified facts from parsed suggestions.
## 3. Scope
Source registry entry, approved API/feed or URL connector, conditional fetch, provenance/content hashes, change detection, duplicate candidates and operator pause.
## 4. Out of Scope
Unrestricted crawling, bypassing access controls, silently merging duplicates or changing verified bid facts.
## 5. Technical Requirements
Approve missing source-table and API cards before handler code; DNS/IP/redirect/egress validation; size/time/rate limits; external credentials protected; malformed/removed source state visible; material change emits review-required event.
## 6. Proposed Implementation Approach
Review source terms; create recorded fixtures; implement fetch/normalize port; preserve raw immutable references; bounded scheduling/backoff; expose freshness and candidate verification flow.
## 7. Dependencies
[Future scope](../architecture/BID-ARC-010.md), [jobs](../events_workers/BID-EVT-002.md), [safe documents](../documents/BID-DOC-002.md).
## 8. Acceptance Criteria
- Given redirect/private IP or DNS rebinding, then fetch is denied.
- Given source changes, then provenance/history remains and linked bid is not silently rewritten.
- Given 429/outage, then retry respects limits and freshness warns user.
## 9. Testing Requirements
Recorded-source contracts, SSRF, parser/version, duplicate/change and stale-source tests.
## 10. Definition of Done
Approved connector and safe import evidence, monitoring and owner; later release only.
## 11. References
[BR-030–034](../../product/02_BRD.md), [integrations](../../technical_design/16_INTEGRATION_ARCHITECTURE.md), [integration plan](../../implementation/13_INTEGRATIONS_IMPLEMENTATION.md), [AGENTS](../../../AGENTS.md).
## 12. Risks and Notes
Live-source checks are separate from deterministic fixture tests.
