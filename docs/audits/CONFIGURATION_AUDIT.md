# Configuration enforcement audit

## Result and evidence boundary

Source audit against `a690c55` plus the existing dirty working tree. Fixes are local, **not deployed**. No migration, live settings mutation, real email, external OAuth, billable AI request, payment or provider activation was performed.

**The full end-to-end audit is not complete.** Discovery, family-level consumer mapping and the verified fixes below are delivered. Remaining partial entries and integration blockers must not be reported as verified configuration coverage.

| Source entries | Count |
|---|---:|
| Typed configuration fields | 321 |
| Environment names/fragments | 168 |
| Nginx/Next configuration directives | 30 |
| **Total** | **519** |

| Classification | Count | Meaning |
|---|---:|---|
| Working | 72 | Relevant unit/fake behavior covered; per-row limits apply, not live acceptance |
| Fixed | 18 | Configuration entries implicated in the regression-tested fixes below |
| Partial | 426 | Static/mixed evidence; complete effective runtime acceptance not established |
| Unused | 3 | Explicitly ignored compatibility fields; not restored as configuration sources |
| Broken | 0 | No remaining product defect positively classified in the inventory; this is **not** proof that partial entries work |

These are **source entries, not unique business settings**. Bindings, immutable snapshots, observational metadata and infrastructure controls are included. No live end-to-end coverage percentage is claimed.

## Deliverables and reproducibility

- [`configuration-matrix.csv`](configuration-matrix.csv): every discovered entry, declaration path/line, expected consumer, actual-use evidence, status, issue, fix, verification, scope and up to 20 reference candidates per row (full candidates in JSON).
- [`configuration-inventory.json`](configuration-inventory.json): machine-readable discovery, classifications and totals.
- `scripts/configuration-inventory.go`: names-only discovery; does not read real `.env` files or export configuration/secret values.
- `scripts/configuration-audit.py`: explicit family mappings, classifications and evidence boundaries.

From the repository root:

```sh
go run ./scripts/configuration-inventory.go -root . -out /tmp/bidora-configuration.json
python3 scripts/configuration-audit.py /tmp/bidora-configuration.json docs/audits
```

Discovery limitations: field-name selector references are **candidates**, not Go type-resolution proof. Dynamic environment families are identified but not exhaustively expanded for arbitrary deployment keys. Third-party inherited defaults, arbitrary tenant data and every nested deployment directive are not exhaustively enumerated. Family mapping is code tracing, not proof that each setting completed UI → database → real provider execution. Examples-only names remain Partial because dynamic/deployment use cannot be disproved by a text search.

## Implemented fixes

| Defect | Correction | Regression evidence |
|---|---|---|
| Login, Google linking and password-change persistence validated ordinary bounds against fixed 30-minute/12-hour defaults | Validate supplied bounds against the immutable current `securitypolicy` snapshot; retain same-transaction security rechecks | `tests/identity/configured_session_writes_test.go` |
| Sensitive actions inferred authentication time by subtracting 12 hours from expiry | Project trusted `identity.sessions.created_at` as `AuthenticatedAt`; carry it through the validated principal. Session revocation, proposal approval, submission confirmation, membership offboarding and MFA readiness/primary freshness use that timestamp, not expiry or refreshed assurance | `tests/identity/session_authenticated_at_test.go`, `tests/httpapi/session_expiry_recency_test.go`, updated review/approval and MFA fixtures |
| SMTP delivery/test collapsed three saved time budgets into their minimum; adapter rejected otherwise-valid saved command/overall ranges | Separate connect, command and overall limits; reset command deadlines and cap them by caller/overall context. Managed delivery and test pass all three. Explicit legacy adapter Timeout remains compatibility-only | `tests/identity/smtp_configured_timeouts_test.go`, `smtp_deadline_enforcement_test.go`; real local synthetic SMTP plus stalled TCP/caller-cancellation checks |
| Tenant invitations ignored managed notification template content/revision | Enable managed `tenant.invitation` rendering in runtime composition, record its actual revision/fingerprint and fail closed on resolver errors | `tests/notification/tenant_template_settings_test.go`; replay fence prevents SMTP in this unit test |
| Maintenance/registration/upload gate used path matching inconsistent with case-insensitive routing | Centralize case normalization and trailing-slash handling before enforcing current policy | `tests/platformsettings/operational_gate_test.go` |
| PesaPal order descriptions ignored the configured merchant name | Build description from the exact pinned configuration's merchant name | `tests/platformsettings/pesapal_runtime_test.go`, fake HTTP only |
| SMTP permission description still described an activation request | Display “Apply email settings” | TypeScript, affected ESLint and frontend build |

Also reconciled the existing dirty-tree notification delivery query checksum after reviewing its added `AMBIGUOUS` attempt count, matching generated scan and fail-closed replay consumer. Those existing behavior changes were preserved, not replaced.

`queries/identity/session.sql`, its sqlc-generated reader and its reviewed query checksum changed together. **No database schema change is required.** Missing/future session authentication timestamps fail closed in the real session validator. Existing session bounds and revocation state are not extended or resurrected. Sensitive-action recent authentication and assurance remain separate from session lifetime.

## Principal consumer routes and precedence

Paths below are relative to `backend/` unless prefixed `web/`. The CSV contains declaration and candidate-reference lines.

| Family | UI/API → storage → effective consumer | Evidence boundary |
|---|---|---|
| Platform information | Platform settings form → information service/store and `queries/platformsettings/information.sql` → shared Platform identity and notification branding | Save/read tests and static consumption trace; static assets/page metadata are not dynamically replaced |
| Security/session controls | Security form/BFF → session-policy handlers/store → `identity/securitypolicy` snapshots and transaction rechecks → issuance, persistence, session read/touch, recovery, password rules, attempt budget and multi-session admission | Unit/fake tests; current DB/Redis/browser acceptance remains separate |
| Google | Google settings → encrypted configuration/secret store → `pilot/managed_google.go` → OIDC exchange/claims and exact current signup/domain policy | Revision fencing and fake/unit tests; deployment callback/trust availability remains a prerequisite; no real OAuth |
| SMTP | SMTP form/BFF → direct audited save/vault → `platformsettings/smtpdelivery.Active.WithActive` → identity, tenant and Platform mail; tester uses same typed active parameters | Local synthetic TLS mail, unit authorization/credential retention; no real inbox/Owner acceptance |
| Notification templates | Template UI → notification template service/store → `Render` → identity, Platform-ledger and now tenant invitations → durable redacted/encrypted message/attempt evidence before SMTP | Unit composition/privacy/replay checks; no real inbox rendering |
| Countries | Country UI → regional service/store → country runtime references → organization creation/default country/locale choices | Save/reference validation traced/tested; existing organization/legal billing snapshots are not rewritten |
| Operational flags | Regional settings → `CurrentPolicy` → normalized request gate, upload admission, OpenAI resolution, new PesaPal work and restricted metadata readers | Case/boundary and AI resolver tests; physical retention purge and all real workers remain Partial |
| OpenAI | Draft/test/approval/activation → exact active store → `openairuntime.Resolve` → managed document/response/proposal extractor | Fresh model/timeouts/concurrency/flags/key/root references tested. Expected-provider-model fence rejects changes between reservation and execution. No paid request |
| PesaPal | Draft/test/approval/activation → active version → `Current`/`CreateOrder`; reconciliation uses pinned historical version | Fake HTTP verifies environment/host/money/evidence boundaries; no real payment |
| Plans/entitlements | Published catalogue/tenant accepted terms → immutable subscription/term limits → licences, storage, workspaces, document versions and AI reservations | Dirty-tree catalogue work differs from scoped release; commercial DB integration remains unverified |
| Tax/financial terms | Effective-dated legal profile and accepted invoice/term snapshots → existing billing/accounting | KE/KES legal scope is independent of country display currency; no financial mutation |
| Infrastructure/tenant controls | Deployment loaders → explicit runtime-mode composition → PostgreSQL, Redis, NATS, MinIO, ClamAV, measurement executables, TLS/origin/proxy boundaries | Constructor/unit/code trace, not deployed acceptance |

Global mutable business settings are authoritative where supported. Deployment trust, provider availability/live-payment ceilings and resource/security limits are legitimate independent bounds. Tenant snapshots are intentionally stable. A disabled new-payment configuration must not prevent reconciliation of previously pinned orders. Health-only, identity-only and full-business compositions intentionally mount different components.

SMTP applies immediately for an authorized recently verified Owner, without maker/checker approval. Other provider approval requirements remain intact. Ignored compatibility fields are `pilot.Config.OpenAI` and `pilot.GoogleConfig.ClientID`/`ClientSecret`; credentials/model/business policy come from managed settings, not these fields.

## Executed checks

| Check | Result |
|---|---|
| Scoped clean worktree Go build and all 18 non-integration test packages | PASS |
| Scoped non-integration Go vet | PASS |
| Main dirty-tree Go build | PASS |
| Main identity, HTTP, notification and Platform-settings race suites | PASS |
| Main non-integration Go vet | PASS |
| Main non-integration test run | PASS: all 21 packages after reviewing/reconciling the existing notification-query checksum |
| Full scoped `go test ./...` | FAIL: obsolete integration fixtures below do not compile |
| Scoped OpenAPI check | PASS: 205 operations / 222 schemas |
| Main OpenAPI check | PASS: 220 operations / 225 schemas; unrelated source work preserved |
| Main frontend contract check, TypeScript and affected ESLint | PASS |
| Main frontend production build | PASS |
| New SMTP phase/caller deadline and trusted-authentication-time race checks | PASS |

No authenticated browser settings save, new live DB/runtime-role proof, real mail/provider calls or deployment was executed for this audit. Existing earlier delivery proofs are not silently counted as new verification.

## Remaining work / acceptance blockers

1. **Migrate obsolete integration fixtures to canonical managed SMTP**, without restoring `pilot.RegistrationConfig` or `Config.Registration`. Current compile failures: `tests/integration/identity_registration_composition_test.go:61`, `invitation_http_flow_test.go:78`, and `pilot_registration_composition_test.go:32,242`. Then run the complete designated disposable-DB/Redis integration lifecycle, including current mandatory email-code sign-in.
2. Complete per-entry acceptance for the **426 Partial entries**, especially commercial snapshots, country onboarding, physical retention cleanup, deployed storage/scanner/measurement/eventing dependencies, dynamic environment families and framework/proxy directives. Validate long SMTP tests against the complete browser/BFF/proxy/API request budgets, not just transport deadlines.
3. Obtain an authorized disposable Owner/browser workflow for global save → reload → downstream behavior and a separately authorized real inbox/OAuth/provider acceptance plan. Paid activation, real financial transactions and production changes remain unauthorized.

**Next recommended action:** repair managed-SMTP integration fixtures first, then run the disposable integration matrix. The inventory is an evidence ledger, not a declaration that end-to-end configuration enforcement is finished.
