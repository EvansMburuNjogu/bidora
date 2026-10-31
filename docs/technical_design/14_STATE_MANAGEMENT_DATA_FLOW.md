# State management and data flow

Sources: [frontend §§12–13](../product/12_FRONTEND_UX_DESIGN.md), API-004/005; decisions [003, 016](27_ARCHITECTURE_DECISIONS.md). Execution: [frontend](../implementation/10_FRONTEND_IMPLEMENTATION.md).

## State classes

Server state uses typed query hooks (TanStack Query proposed, not installed). Cache keys include identity/session generation, tenant, resource ID and view parameters. URL state carries non-sensitive filters/sort/tab; never tokens, pricing or proof content. Form state stays local with schema guidance (React Hook Form/Zod proposed). Session bootstrap comes from `/v1/me`, never a local hardcoded role. Dialog/selection state is ephemeral.

## Mutation protocol

1. Capture active context generation and expected version.
2. Validate shape locally; server owns business rules.
3. Create one idempotency key for the user's operation; preserve it across ambiguous network retries. A changed payload is a new operation/key.
4. Send context selector, CSRF and version precondition.
5. Apply returned authoritative DTO; invalidate dependent readiness/approval/package queries.
6. On 409, retain permitted draft and offer refresh/compare. Never silently resend against a newer version.

Do not optimistically show APPROVED, SUBMITTED, CLEAN or package success. Cosmetic task selections may be optimistic if reverted on failure. A 202 response creates a durable job query, not a successful business transition.

## Workspace switch

Block or confirm unsaved edits; cancel old requests; increment context generation; clear tenant-scoped caches; bootstrap selected context; ignore results stamped with older generation. Go additionally validates request context against membership/session. Multi-tab mismatch receives CONTEXT_STALE and requires explicit refresh rather than routing a command to the new tenant.

## Sensitive rendering

Private Next.js fetches are request-scoped/no-store; no global singleton cache of `/me` or tenant DTOs. Server component serialization must omit forbidden pricing, not rely on client-side hiding. Logout removes sensitive query/form state and revokes server session. Browsers may retain downloaded files; the product must not claim recall of already exported evidence.

## Polling and offline

Use bounded polling with jitter and backoff for upload/package jobs; stop on terminal state, logout or lost permission. No offline queue for approval/submission commands. Draft offline persistence is excluded until encryption, retention and device-risk policy are approved. On reconnect revalidate versions before any write.

## Verification

Race two workspace switches against slow requests; revoke membership while package job runs; simulate response loss after committed command; verify retry returns the same result without duplicate events. Test back button, stale tabs, logout, 401/403/404/409/429 and server/network errors with preserved safe form input.
