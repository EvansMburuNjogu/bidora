# BID-AUTH-003C — Platform Administrator invitation lifecycle

## Metadata
- Ticket ID: BID-AUTH-003C
- Epic: E-AUTH
- Type: Engineering
- Priority: P0
- Status: IMPLEMENTED_NOT_VERIFIED
- Complexity: M
- Dependencies: BID-AUTH-003, BID-ARC-011, BID-API-010
- Blockers: Real PostgreSQL race/rollback evidence, SMTP composition, HTTP/browser composition and R5 architecture tests
- Milestone: M7

## 1. Objective
Allow an effective Platform Owner to issue and cancel fixed-role invitations and allow an eligible recipient to accept one without crossing module or cell boundaries.
## 2. Business Context
Platform administration needs controlled entry, exact fixed roles and reliable recipient communication without email matching becoming authority.
## 3. Scope
Application-owned issue/cancel/accept orchestration; recent TOTP AAL2; exact email/no-tenant-ties acceptance; hash-only 32-byte tokens; 72-hour expiry; resend replacement; owner continuity exception/pending approval handoff to BID-AUTH-003D; auth-epoch/session effects; atomic command/audit/outbox facts; provider-neutral professional invitation/reminder templates.
## 4. Out of Scope
HTTP, SMTP provider composition, reminder scheduling, maker/checker approval HTTP handlers, migration execution, bootstrap command, cell infrastructure, web and deployment.
## 5. Technical Requirements
Platform Administration remains global control-plane work. Account type, role labels, email and context generation never independently authorize. Application owns sequencing through abstract units of work; Identity and Authorization own their data access. Raw tokens exist only for immediate delivery and are never persisted/logged/returned after acceptance.
## 6. Implementation Approach
Extend the BID-ARC-011 facade with invitation services, transaction-scoped owner adapters and shared control evidence. Render minimal multipart-ready text/HTML without remote assets or tracking; leave transport and reminder cadence explicit deployment/product follow-ups.
## 7. Acceptance Criteria
Issue/cancel require current effective owner plus TOTP AAL2 no older than 15 minutes; acceptance requires exact eligible identity and active TOTP; replay/conflict fail safely; accepted privilege/session/evidence effects are atomic; owner-invitation exception cannot silently reopen.
## 8. Verification
Deferred centralized application tests and real PostgreSQL resend, expiry, competing acceptance, owner-exception, idempotency and rollback races; SMTP rendering/delivery checks later.
## 9. References
[BID-ADR-024](../../../backend/docs/cell-ready-hexagonal-architecture.md), [P1–P6](../../../backend/docs/platform-admin-authority-decision-packet.md), [contract](../../../backend/api/openapi/drafts/platform-administrator-lifecycle.json).
