> **Implementation history/reference.** Maintain current MVP work in [the single delivery tracker](../../docs/tickets/PLATFORM_ADMIN_DELIVERY.md). Dated scope/approval restrictions below are historical; [root instructions](../../AGENTS.md) govern current full-stack development.

# Bidora web implementation tickets

Statuses distinguish source implementation from verification, UAT, deployment and acceptance.

## WEB-AUTH-002 — Session bootstrap and backend-authoritative portal resolution

**Persona/outcome:** an authenticated user completes sign-in and reaches only the portal destination returned by the current backend authority check.

### WEB-AUTH-002A — Contract projection and BFF

Status: **IMPLEMENTED_NOT_VERIFIED**

- Generate selected TypeScript contracts from the canonical backend OpenAPI document.
- Proxy only the approved CSRF bootstrap and `GET /v1/me` reads.
- Keep backend and approved browser origins server-only and explicit.
- Preserve cookies, safe statuses, trace headers and private no-store behavior.
- Excludes account-type mutation, membership selection and authority inference.

### WEB-AUTH-002B — Memory-only session runtime

Status: **IMPLEMENTED_NOT_VERIFIED**

- Restore the session-bound CSRF token into React memory only.
- Parse bootstrap and portal responses fail-closed against generated contract values.
- Redirect an invalid session to sign-in and provide bounded retry for unavailable dependencies.
- Never use localStorage, sessionStorage or a frontend-decoded cookie as authority.

### WEB-AUTH-002C — Portal resolver and guarded entry boundaries

Status: **IMPLEMENTED_NOT_VERIFIED**

- Navigate only to the exact backend-returned allowlisted destination.
- Re-resolve direct Tenderpreneur or Platform entry before rendering the boundary.
- Preserve required-action states without manufacturing membership or capability.
- Keep Procurement unavailable without creating a placeholder Procurement route.
- Portal entry boundaries contain no dashboard data or business-operation claims.

## WEB-AUTH-003 — Tempo-style sign-in adaptation

Status: **IMPLEMENTED_NOT_VERIFIED**

Adapts the public Tempo sign-in composition into a product-owned Bidora page: a compact 22rem authentication column, responsive 600px contextual panel, restrained dotted field, email/password-first form, provider separator and Google action. Demo magic-link behavior, Apple/GitHub providers, testimonials, customer logos, localStorage sessions and licensed source are excluded. The contextual panel uses source-defined Bidora principles rather than invented endorsements.

## WEB-AUTH-004 — Complete authentication experience and Google callback bridge

Status: **IMPLEMENTED_NOT_VERIFIED**

Standardizes registration, verification, password recovery/reset, MFA continuation, session restoration, and Google completion/failure states on the Harbor Teal authentication composition. Every authentication input has explicit placeholder guidance; every password field has an accessible show/hide control. Registration now captures required first and last names plus work email/password, requires an exact client-side password confirmation, and places correction beside the affected field. Only the approved first-name/last-name/email/password payload crosses the BFF; confirmation remains client-side. MFA supports both six-digit TOTP and one-time recovery codes. Password and Google registration/sign-in continue through the existing bounded BFF, with a dedicated no-store Google callback bridge that forwards only cookies and fixed backend redirects. External Google credentials, callback registration, provider acceptance, composed browser testing, accessibility acceptance, and deployment activation remain unverified.

## WEB-AUTH-005 — Plain-language content and Go contract alignment

Status: **IMPLEMENTED_NOT_VERIFIED**

Replaces implementation-focused wording across authentication, account access, organization selection, and the Tenderpreneur entry shell with concise user-facing language, using “AI-powered” sparingly. Google actions use the recognizable multicolor Google “G”. Authentication requests are typed per Go operation so each form can submit only the exact backend fields: `email`/`password`, `email`, `token`/`new_password`, `token`, or `code`/`method`. Client guidance mirrors the Go email, password, email-code, authenticator-code, and recovery-code rules while the Go backend remains authoritative. Live composed browser verification remains pending.

## WEB-AUTH-006 — ReUI Sonner authentication feedback

Status: **IMPLEMENTED_NOT_VERIFIED**

Loads the current ReUI agent skill through the configured MCP workflow and adopts the installed shadcn Sonner component for submitted authentication outcomes. Registration and recovery now show short success toasts before moving directly to code entry; verification and reset return to sign-in with a success toast; backend rejection, rate limiting, dependency failure, and Google completion failure use error toasts. Toast surfaces now explicitly use popover background/foreground tokens, readable muted descriptions, semantic borders/icons and contrasting close controls in light and dark themes. Field correction remains beside the field. The temporary Playwright dependency was removed at the user’s request; browser automation remains deferred.

## BIDORA-BRAND-002 — Bidora Aperture identity

Status: **VERIFIED_TECHNICALLY** — visual direction approved and refreshed logo/favicon visibility confirmed by the requesting user; trademark similarity review remains pending.

Introduces the Bidora Aperture: two precision-cut planes converge around a protected diamond-shaped negative space, representing source and response meeting at a human decision point. The non-letter mark uses Deep Harbor (`#0A3438`) and warm ivory (`#F7F4EC`) with balanced primary/inverse variants. Versioned `bidora-aperture` source SVGs are exported as 192 px and 512 px transparent PNG images, used with a tightened Bidora wordmark, and supplied as a filled Next.js application icon, 180 px Apple icon, and 32 px ICO favicon. It contains no motion and preserves standard browser zoom. Trademark similarity review remains pending.

## WEB-MOBILE-001 — Authentication and MFA mobile usability

Status: **IMPLEMENTED_NOT_VERIFIED**

The active authentication, account-security and MFA enrollment surfaces now reflow without a secondary marketing panel below desktop width, use 16 px mobile inputs, larger primary controls, wrapping field-label action rows, full-width narrow-screen MFA actions, break-safe secrets/recovery codes and shrinking page/card parents. Display and title tokens permit emergency wrapping for long localized or organization content. Behavior, contracts and security boundaries are unchanged. Safari/mobile/200% visual acceptance remains manual.

## WEB-ONBOARD-001 — Tenderpreneur self-onboarding design

Status: **DESIGNED**

`docs/tenderpreneur-self-onboarding-plan.md` defines the backend-authoritative account → verification → deliberate Tenderpreneur selection → invitation or organization creation → security → workspace journey. Existing registration, verification, account-type, invitation, MFA, `/v1/me` and membership contracts are mapped. The subsequently approved organization-creation decisions and unmounted intake contract are consumed by WEB-ORG-001; provisioning, payment and activation remain separate blocked operations.

## WEB-ORG-001 — Tenderpreneur organization-creation request

Status: **IMPLEMENTED_NOT_VERIFIED / BACKEND UNMOUNTED**

Adds the responsive authenticated `/organization/create` form and same-origin server-only BFF for the approved `POST /v1/tenderpreneur/organizations` contract. The fixed Tenderpreneur type requires an explicit confirmation and Trial/Paid has no default. Session cookies and memory-held CSRF are forwarded only by the BFF; form state and UUID idempotency remain in browser memory and are never placed in browser storage. A canonical `202 REQUESTED` is represented only as a pending request, with explicit language that provisioning, workspace access, Trial and payment are not complete. The BFF maps the intentionally unmounted backend route to a safe unavailable response. Backend activation, provisioning workers, payment calls, Trial activation, functional tests, browser acceptance and UAT are excluded.

## WEB-AUTH-009 — MFA enrollment

Status: **IMPLEMENTED_NOT_VERIFIED**

Completes canonical MFA readiness/enrollment/activation DTOs and generated frontend projections, then adds a fail-closed account-security flow for readiness, pending TOTP setup, six-digit confirmation and one-time display of exactly ten recovery codes. TOTP secrets, setup URIs, recovery codes and CSRF values remain memory-only and are never logged or persisted by the frontend. Confirmation forwards replacement cookies but defers the in-memory session refresh until the user confirms the recovery codes are saved, preventing their one-time response from disappearing during rotation. A recent primary sign-in is required before enrollment.

The bounded local identity runtime is available at `127.0.0.1:8080` against the approved synthetic TLS PostgreSQL/Redis fixtures; the Next.js BFF is available at `127.0.0.1:3000`. One synthetic seeded login is stored outside the repository in the mode-0600 integration credential file. Login, bootstrap, portal resolution and MFA-preparation smoke requests passed; MFA activation remains for manual browser verification.

## WEB-AUTH-008 — Current-password change

Status: **IMPLEMENTED_NOT_VERIFIED**

Adds authenticated `/account/security` navigation and a strict current-password form for canonical `POST /v1/auth/password`. The BFF accepts only the approved `email`, `current_password` and `new_password` JSON payload, exact browser Origin, session cookies and the memory-held CSRF value. Successful backend session/CSRF rotation is forwarded to the browser, the form is cleared, other-session revocation is disclosed, and `SessionProvider` immediately bootstraps the replacement CSRF value without persisting it. The shared account route is available to authenticated Tenderpreneur and Platform users without treating portal paths as authority. Live backend/browser verification and deployment trusted-proxy evidence remain pending.

## WEB-AUTH-007 — Backend-aligned authentication completion

Status: **IMPLEMENTED_NOT_VERIFIED** (logout slice); remaining operations are sequenced below.

Audited the canonical OpenAPI, Go route composition and handlers against the Next.js BFF and active authentication screens. Registration, verification, password login, login MFA challenge, password recovery/reset, Google sign-in callback, CSRF bootstrap, `/v1/me`, active memberships and current-session logout now have frontend paths. Logout forwards only the exact Origin, cookies and memory-held 43-character CSRF value; it rejects query/body input, forwards backend cookie clearing and returns users to sign-in without persisting authentication state.

Next eligible operation is current-password change because its canonical request is fully defined and its success response has no body. MFA preparation/enrollment/confirmation/replacement/recovery-code management and session inventory cannot yet receive fail-closed generated frontend DTOs: their canonical 200/201 responses currently describe behavior but provide no response content schemas. Google account linking remains blocked by the explicit takeover-prevention policy gate, and irreversible account-type selection remains deferred.

## WEB-DESIGN-002 — Application-wide typography and content system

Status: **IMPLEMENTED_NOT_VERIFIED**

Replaces Inter with variable Manrope and establishes centralized responsive display, title, section, lead, body, meta, kicker, wordmark, page-gutter, section-gap, copy-measure and line-height tokens in `src/app/globals.css`. All active authentication, session resolution, organization selection, Platform entry and Tenderpreneur shell/overview surfaces now consume the semantic type and page-layout classes rather than isolated heading sizes. Content was audited for concise sentence case, consistent sign-in terminology, shorter descriptions, predictable action labels and reduced duplicate status messaging. Existing ReUI/shadcn component sizing and application behavior remain unchanged. Lint, TypeScript, build, production route rendering, ReUI C1–C4 and the ReUI audit checklist pass; browser zoom, forced-colors, screen-reader and visual acceptance remain pending.

## WEB-DESIGN-001 — Harbor Teal semantic palette

Status: **IMPLEMENTED_NOT_VERIFIED**

Replaces the neutral black/white foundation with the selected Harbor Teal system: warm ivory canvases, blue-black text, deep teal primary actions, sea-glass contextual surfaces, and a deep blue-green sidebar. Success, information, warning and destructive colors remain separate semantic state tokens. Representative computed contrast ratios are recorded in `web/docs/visual-palette.md`; browser and accessibility acceptance remain deferred.

## Follow-on tickets

### WEB-ONBOARD-001 — One-time account-type selection

Status: **DESIGNED / DEFERRED**

Implement the approved idempotent account-type mutation with explicit consequences. Do not enable it until the user experience addresses irreversible choice and the unavailable Procurement portal.

### WEB-PORTAL-001 — Tenderpreneur shell and active organization context

Status: **IMPLEMENTED_NOT_VERIFIED**

Consumes `GET /v1/me/memberships`, requires an explicit current membership choice for every fresh browser view, and keeps tenant ID/context generation in React memory as selectors only. Directory refresh preserves the choice only when membership, tenant and generation still match; revocation or generation change clears it and advances a local selection epoch. The responsive dark-sidebar shell contains only a real organization switcher and the implemented Overview route. No server-persisted context, default tenant, fabricated dashboard data or business authority is introduced.

### WEB-PORTAL-002 — Platform Administration shell

Status: **DESIGNED / DEFERRED**

Use the backend-returned platform context generation and capabilities as display/routing inputs only; every operation must retain backend authorization and recent-assurance checks.

### WEB-QA-001 — Auth/session/portal verification

Status: **DEFERRED**

Add centralized contract and Playwright coverage for 401/403/503, malformed DTOs, CSRF secrecy, direct-route mismatch, required actions, stale sessions, keyboard behavior, 200% zoom and accessibility. Real composed acceptance must use the Go backend and synthetic dependencies.
