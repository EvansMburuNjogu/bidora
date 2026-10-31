# Contract implementation workstream

## 1. Purpose
Make approved domain operations machine-verifiable across Go, TypeScript and workers.

## 2. Scope
OpenAPI 3.1, event/AsyncAPI schemas, DTO generation, examples, errors and compatibility checks. Phase 4 is governance, not permission to code handlers before contracts.

## 3. Product specifications
[API design §18](../product/11_BACKEND_API_DESIGN.md), API-001–008, SR-EVT-003.

## 4. Technical designs
[API cards](../technical_design/10_API_CONTRACT_DESIGN.md), [event contracts](../technical_design/17_EVENT_MESSAGING_DESIGN.md).

## 5. Prerequisites
Actor, state, policy, data classification, domain invariants and acceptance cases approved for the operation.

## 6. Required decisions
Identity ADR-007; policy ADR-011; deadline/exchange ADR-009/010; proposed API selector/error codes need explicit contract review.

## 7. Dependencies
Contract draft → security/domain review → schema approval → handler/client implementation → conformance. Read-only candidate/history/session endpoints noted as gaps require cards first.

## 8. Target files
Future backend/api/openapi, backend/api/events, generated Go DTOs and `<frontend-root>/src/contracts`, fixture examples and compatibility tests.

## 9. Sequence
Define required/nullability/limits; headers and auth; errors and audit; pagination; examples; generate types; bind server validation; centralize frontend errors; run producer/consumer tests; freeze baseline before release.

## 10. Domain/security
No secret/object locator properties; sensitive fields omitted before serialization. Idempotency replays reauthorize. Tenant selector never grants authority. Event actor is provenance, not credentials.

## 11. Database/API/frontend
Expected versions correspond to owning aggregate; decimals strings; immutable IDs/version refs; 202 returns durable job Location. No frontend-generated authoritative states.

## 12. Tests
Validate examples, malformed/unknown fields, stale version, reused key/different payload, forbidden fields, pagination stability, 401/403/concealed 404 and old-client compatibility.

## 13. Acceptance
Generated clients and real responses conform; deliberate breaking change fails CI; errors are stable and safe.

## 14. Done
Approved schema, reviewed examples, reproducible generation and passing conformance evidence.

## 15. Risks
Adding enum values can break exhaustive consumers; do not call every additive change safe. Avoid generated DB models as wire contracts.

## 16. Evidence
Schema digest/version, compatibility diff, generator version, tests and reviewer sign-off.

## 17. Exclusions
Runtime handlers or schema files in this documentation task; future endpoint implementation from names alone.
