# Bidora — current development instructions

## Scope and authority

Build the full-stack SaaS MVP using the existing `backend/` Go application and `web/` Next.js app. The current delivery source of truth is [Platform Admin delivery](docs/tickets/PLATFORM_ADMIN_DELIVERY.md): exactly five BID-MVP parent tickets, with implementation tasks beneath them.

The user's MVP delivery reset supersedes historical backend-only, health-only, deferred-testing and routine approval restrictions. Earlier instructions are preserved in [the historical snapshot](docs/history/agent-instructions-before-mvp-reset.md); that snapshot is reference, not an active instruction file. Historical tickets and evidence describe their dated scope, not today's permission to develop. Do not rewrite past decisions or treat old unresolved implementation approvals as a reason to stop ordinary development.

## Proceed without repeated approval

Implement frontend/backend functionality, endpoints and compatible contracts, forward-compatible migrations, safe refactors, components, fixtures, tests, generation and local builds. Rebuild/restart clearly identified Bidora-owned development services when needed. Use reasonable engineering defaults and existing conventions. Missing production credentials do not block source implementation or unrelated work.

Read the requirements, contracts and relevant code for the operation, not every historical document before every edit. Preserve established business invariants. Resolve ordinary engineering conflicts using the latest explicit direction; ask only when a decision actually crosses a boundary below. Continue independent operations if one dependency is unavailable.

Explicit authorization is still required for production deployment/data changes, destructive operations against existing databases, real financial transactions, external spending/production provider activation, weakening mandatory security or isolation, and legally binding policy decisions. Permission to write code is not permission to operate production systems.

## Security and data boundaries

- Default deny. Enforce current authentication, exact capabilities, assurance, CSRF/Origin and server-validated context. Platform administration does not imply access to tenant-private data. Preserve maker/checker and last-admin safeguards where applicable.
- Resolve tenant databases only from authorized membership and the server registry; no caller-selected DSNs, fallback tenant or cross-tenant joins. Keep tenant-private bids/pricing/evidence outside control/public stores.
- Never commit, log or return secrets, raw tokens, private documents or unnecessary personal data. Provider secrets require encrypted storage with deployment-supplied key material and restricted write-only/redacted interfaces; no plaintext settings store or UI secret echo. Do not activate paid providers merely to verify code.
- Keep immutable audit/evidence history; corrections append history. Do not fabricate data or claim delivery, payment, submission or acceptance without evidence. Restricted IP/browser audit metadata has a provisional configurable **90-day development retention**, separate from immutable events; production retention/legal holds require confirmation before activation.
- Documents remain quarantined until fail-closed scanning confirms the exact version CLEAN; verify size/hash and sandbox parsing. AI is untrusted advisory output with exact citations and human acceptance, never an approval/submission/award authority.
- Only a clearly identified disposable or expressly designated development database may receive integration-test writes. Never assume an existing database is safe. Preserve unrelated host services/databases; avoid destructive resets and automatic down migrations.

## Implementation conventions

- Reuse Go/Fiber, pgx/sqlc/PostgreSQL, Goose, and Next.js App Router/TypeScript/Tailwind with installed ReUI/shadcn components. No replacement framework, duplicate frontend root or substitute bootstrap/tooling project. Preserve the generated Next.js agent block in `web/AGENTS.md` and installed design-system appearance.
- Keep domain/application logic behind owner-module ports; HTTP binds/validates and invokes use cases. Do not call another module's repositories directly. Application transactions own state, audit, outbox and idempotency in one database; cross-plane effects require durable reconciliation.
- Use parameterized SQL, bounded/cancellable external calls, optimistic versions and safe replay semantics. Do not hold database transactions over provider calls. Preserve truthful provider outcomes and fail-closed dependency errors.
- Update API contracts and generated artifacts with affected code; never hand-edit generated output. Use safe stable errors, snake_case JSON, decimal money plus currency and explicit UTC/timezone handling.
- Connect complete workflows: UI → API → permission → operation → persistence → audit/event → refreshed UI. No fake success, fabricated metrics or forms presented as saved when they are not. Keep private responses no-store, session secrets out of localStorage, and context changes from showing stale private data.
- Preserve archived local-fixture experiments without importing/restoring them into the application. Execute manually; do not launch subagents without explicit delegation authorization. Preserve any existing agent configuration/usage ledgers.

## Checks and delivery

Run affected formatting, compilation, contract-generation and focused tests during implementation. Go tests belong under `backend/tests/` in external packages; frontend tests belong under `web/tests/`. Do not weaken existing tests. Use real disposable/designated dependencies for persistence/isolation and end-to-end evidence; mocks and compilation are not live verification. Broader browser, accessibility, security and integration verification belongs to BID-MVP-05, without preventing focused checks now.

Keep implementation completion, verification, provider readiness and production acceptance separate. Close implementation tickets at their stated finish line; do not reopen them for unrelated improvements or missing production credentials. Fix bugs in the responsible area. Update the one tracker with concise actual evidence and the next actionable task; do not create another delivery programme or recurring approval process.

Report briefly: **Ticket**, **Fixed**, **Code Changed**, **Checks**, **Next**. State what actually ran and what remains unverified. Continue actionable work without asking for another broad planning instruction.
