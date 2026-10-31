# BID-AUTH-003E — Fixed Platform Administrator role lifecycle

## Metadata
- Ticket ID: BID-AUTH-003E
- Epic: E-AUTH
- Type: Engineering
- Priority: P0
- Status: IMPLEMENTED_NOT_VERIFIED
- Complexity: M
- Dependencies: BID-AUTH-003D, BID-ARC-011, BID-API-010
- Blockers: Owner-role disposition execution, real PostgreSQL race/rollback evidence, HTTP/notification composition and R5 tests
- Milestone: M7

## Objective
Create exact fixed-role assignment and revocation changes under current effective-owner authority.

## Scope
Non-owner role changes execute atomically; Platform Owner role changes become exact 30-minute maker/checker proposals. Enforce no self-grant, checked-only voluntary owner relinquishment, expected profile/context/assignment versions, final-owner preparation, auth-epoch/session invalidation, idempotency, audit/outbox and minimal role-change notification templates.

## Exclusions
HTTP, migration execution, services, SMTP wiring, web, custom roles, identity disable/reactivation and owner-change checker execution.

## References
[BID-ADR-024](../../../backend/docs/cell-ready-hexagonal-architecture.md), [P1–P6](../../../backend/docs/platform-admin-authority-decision-packet.md), [contract](../../../backend/api/openapi/drafts/platform-administrator-lifecycle.json).
