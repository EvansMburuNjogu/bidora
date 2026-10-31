# BID-AUTH-003D — Govern Platform Owner invitation disposition

## Metadata
- Ticket ID: BID-AUTH-003D
- Epic: E-AUTH
- Type: Engineering
- Priority: P0
- Status: IMPLEMENTED_NOT_VERIFIED
- Complexity: M
- Dependencies: BID-AUTH-003C, BID-ARC-011, BID-API-010
- Blockers: Real PostgreSQL maker/checker race and rollback evidence, SMTP/HTTP composition and R5 architecture tests
- Milestone: M7

## Objective
Complete the exact maker/checker approval or rejection path for pending Platform Owner invitations.

## Scope
Distinct eligible checker, recent TOTP AAL2, exact change version, maker/recipient separation, current effective-maker recheck, 30-minute expiry, approval-time token generation, rejection invalidation, idempotency, audit/outbox and post-commit provider-neutral delivery.

## Exclusions
HTTP, migration execution, service startup, SMTP deployment, reminder scheduling, web work and broader role/identity lifecycle changes.

## Acceptance boundary
Approval activates only the exact pending owner invitation for 72 hours; rejection leaves no usable token. Raw tokens exist only for post-commit delivery. Any stale authority, version, expiry, self-check or recipient-check attempt fails closed. Verification is deferred.

## References
[BID-ADR-024](../../../backend/docs/cell-ready-hexagonal-architecture.md), [P1–P6](../../../backend/docs/platform-admin-authority-decision-packet.md), [contract](../../../backend/api/openapi/drafts/platform-administrator-lifecycle.json).
