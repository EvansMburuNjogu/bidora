# TenderOS — Frontend Template Adoption and Reverse-Engineering Guide

## 1. Purpose

A downloaded frontend template will accelerate visual implementation, but it must be treated as an input to a governed product design process. It must not define business workflows, roles, routes, fields, or backend behavior.

The template has not been provided with this specification pack. The steps below become mandatory when it is added to the workspace.

## 2. Required inputs

- Original template archive/repository
- License and purchase/download evidence
- Template documentation
- Framework/package manifest and lockfile
- Asset/font/icon licenses
- Screenshots or source demo URL if available
- Product brand name/logo/colors if decided

Do not distribute proprietary template or font files outside their license terms.

## 3. Stage A — Safety and license audit

Before running the template:

- Verify source and checksum where available
- Review license for commercial SaaS use, redistribution, seat/project limits, and attribution
- Scan archive and dependencies
- Inspect scripts, postinstall hooks, remote assets, analytics, trackers, and embedded credentials
- Identify abandoned, vulnerable, or duplicate packages
- Confirm framework version compatibility
- Quarantine unknown binaries

Output: `docs/design/template-audit.md` and accept/reject decision.

## 4. Stage B — Structural inventory

Inventory:

- Routes/pages
- Layouts and navigation variants
- Components and component variants
- Forms and validation
- Tables/charts
- Dialogs/drawers/toasts
- Icons/images/illustrations
- Typography and fonts
- Theme/dark mode
- Responsive breakpoints
- State management and API assumptions
- Mock-data sources
- Third-party plugins

Classify each item:

- Reuse as-is
- Reuse after accessibility/quality correction
- Restyle/rebuild
- Remove
- License-restricted

## 5. Stage C — Extract a design system

Convert template styling into product-owned tokens:

```text
Color
Typography
Spacing
Radius
Borders
Elevation
Layout widths
Breakpoints
Motion
Focus
Component density
Status semantics
```

Map tokens into Tailwind CSS theme variables and CSS custom properties. Avoid scattered hardcoded values.

Create/normalize shadcn-based primitives under repository ownership. Template-specific wrappers must not obscure accessible behavior.

## 6. Stage D — Product route mapping

Map template layouts to actual route groups:

- Public website/catalogue
- Authentication/onboarding
- Platform Admin
- Procurement Team
- Tenderpreneur

For every template page:

- Identify whether an approved TenderOS operation needs it.
- Map it to a route/page contract.
- Remove pages with no product purpose.
- Do not rename a generic CRM/e-commerce page and call it procurement functionality.

## 7. Stage E — Component mapping

Example mapping table:

| Product component | Template candidate | Decision | Required changes |
|---|---|---|---|
| App shell | Sidebar layout A | Adapt | Portal-aware nav, workspace switcher, responsive/a11y |
| Compliance matrix | Generic data table | Rebuild/extend | Grouping, evidence, sticky columns, saved filters |
| Tender authoring | Settings form | Partial reuse | Section navigation, version, review comments, validation |
| Evaluation workspace | None | New | Split evidence/scoring pane, restricted envelopes |
| AI result panel | Chat panel | Rebuild | Sources, confidence, warnings, disposition |
| Document viewer | File preview | Adapt | Version, permissions, page citations, secure URLs |

## 8. Stage F — Remove template assumptions

Remove or replace:

- Hardcoded demo users/roles
- Mock API calls and fake counts
- LocalStorage as business source of truth
- Placeholder charts without defined metrics
- Generic status values
- Frontend-only authorization
- Raw IDs in forms
- URL inputs where file upload is required
- Unnecessary card grids
- Broken mobile navigation
- Inaccessible custom components
- Remote fonts/assets that violate policy or performance goals
- Template analytics/telemetry

## 9. Stage G — Integrate contracts

- Generate or validate TypeScript types from OpenAPI.
- Implement centralized API client and error mapping.
- Bootstrap identity/context from `/v1/me`.
- Build portal navigation from effective capabilities and product route map.
- Use real backend pagination/filter/sort.
- Implement version conflict and idempotency behavior.
- Clear tenant-scoped data on context change.

No accepted route may depend on static mock JSON.

## 10. Stage H — Visual and accessibility validation

Validate:

- Keyboard navigation
- Focus visibility/order
- Screen-reader labels and live regions
- Contrast
- Table/form semantics
- Dialog/drawer trapping and restore focus
- Error summary/field association
- Responsive desktop/tablet/mobile behavior
- Reduced motion
- 200% zoom/reflow
- Long names, currencies, languages, and dates
- Empty/loading/error/forbidden/conflict states

## 11. Stage I — Product screenshot baseline

Capture approved screenshots for:

- Public tender page
- Platform Admin tenant list/detail
- Procurement dashboard and tender authoring
- Clarification and evaluation workspaces
- Tenderpreneur discovery, import, compliance matrix, proposal, readiness, submission
- Mobile essential flows

Use visual regression tests for stable components, not to lock every dynamic data pixel.

## 12. Acceptance criteria

The template adoption is complete when:

- License/security audit is approved.
- Product-owned tokens and component inventory exist.
- Portal route/navigation map is implemented.
- Critical components meet accessibility requirements.
- Mock pages/data are removed from accepted workflows.
- No template component bypasses backend authorization or product rules.
- Performance budget is met.
- Visual design is consistent across all three portals.
- Product/UX owner accepts representative workflows.

