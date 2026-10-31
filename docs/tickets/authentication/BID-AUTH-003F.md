# BID-AUTH-003F — Execute pending Platform Owner role changes

## Metadata
- Ticket ID: BID-AUTH-003F
- Epic: E-AUTH
- Type: Engineering
- Priority: P0
- Status: IMPLEMENTED_NOT_VERIFIED
- Complexity: M
- Dependencies: BID-AUTH-003E, BID-ARC-011, BID-API-010
- Blockers: Real PostgreSQL final-owner/maker-checker races, rollback evidence and HTTP composition
- Milestone: M7

## Objective
Approve and execute, or reject, an exact pending Platform Owner assignment or revocation.

## Scope
Distinct eligible checker, current maker/effective-owner recheck, affected-subject separation, 30-minute expiry, exact target/profile/context/assignment versions, final-owner preservation, auth-epoch/session invalidation, governance generation, idempotency and atomic evidence.

## Exclusions
HTTP, migration execution, service startup, sender wiring, web, identity disable/reactivation and non-role administration.

## References
[BID-ADR-024](../../../backend/docs/cell-ready-hexagonal-architecture.md), [P1–P6](../../../backend/docs/platform-admin-authority-decision-packet.md), [contract](../../../backend/api/openapi/drafts/platform-administrator-lifecycle.json).
