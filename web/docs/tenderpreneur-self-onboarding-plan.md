# Tenderpreneur self-onboarding wizard

Status: **PARTIALLY IMPLEMENTED_NOT_VERIFIED** — account entry and organization-request intake exist; provisioning, activation and composed wizard verification remain blocked.

## Outcome

A verified individual can create a Bidora account, deliberately choose the Tenderpreneur path, join an invited organization or request creation of a new isolated organization, secure the account, and enter an explicitly selected workspace. The wizard never treats account type, organization input, a tenant identifier or a browser step as authority.

## ReUI composition plan

- **Surface:** flow
- **Archetype:** one centered wizard Card, one step visible
- **Container family:** Card only
- **Density:** default; large primary controls on narrow screens
- **Regions:** compact progress, title/description, current fields, inline correction, Back/Continue footer
- **States:** loading, field error, service error with retry, blocked, resumable, complete
- **Motion:** none beyond installed component behavior

At 390 px and 200% zoom, progress is text (`Step 2 of 5`) rather than a horizontal stepper. Actions stack full-width, Back remains available, and no required action is hidden in an overflow menu.

## Proposed journey

### 1. Create account

Existing contracts:

- `POST /v1/auth/register` with `email` and `password`
- `POST /v1/auth/google/start` for Google sign-up/sign-in

Behavior:

- Explain that registration creates an identity only.
- Do not claim organization access, subscription, authority or workspace creation.
- Preserve the generic registration response for existing and new email addresses.

### 2. Verify email

Existing contract:

- `POST /v1/auth/verify-email` with the exact emailed token

Behavior:

- Verification is mandatory before account routing or organization authority.
- Resend remains unavailable until an approved resend contract exists.

### 3. Choose Bidora use

Existing contract:

- `POST /v1/me/account-type`

Proposed Tenderpreneur choice:

- Label: **Prepare tenders with my organization**
- Explain before confirmation that the selection is irreversible under the current backend policy.
- Procurement remains visibly unavailable and must not route to a fabricated portal.
- Require a separate confirmation step; never preselect an account type.

Gate:

- The final irreversible-selection UX needs explicit product acceptance before implementation.

### 4. Join or create an organization

#### Join with an invitation

Existing contract:

- `POST /v1/shared/invitations/accept`

Behavior:

- Accept only the invitation token from the verified matching email.
- The backend creates the approved fixed-role membership and capabilities.
- The wizard re-reads `/v1/me` and `/v1/me/memberships`; it never trusts invitation claims locally.

#### Create a new organization

Approved, unmounted intake contract:

- `POST /v1/tenderpreneur/organizations`

Implemented frontend boundary:

- `/organization/create` requires explicit confirmation of the fixed Tenderpreneur type.
- Trial or Paid is deliberate and has no default.
- The same-origin server BFF forwards session cookies, memory-held CSRF and a memory-only UUID idempotency key.
- Canonical `202 REQUESTED` is shown only as pending; it never claims provisioning, membership activation, Trial start, payment or workspace access.
- While the backend handler remains intentionally unmounted, the BFF fails safely as unavailable and preserves form input in React memory.

Provisioning checkpoints, database/role/secret creation, payment reconciliation, Trial activation, ownership-continuity administration and live route activation remain separate gated work.

### 5. Secure and enter the workspace

Existing contracts:

- `GET /v1/auth/mfa/preparation`
- `POST /v1/auth/mfa/totp/enroll`
- `POST /v1/auth/mfa/totp/confirm`
- `GET /v1/me`
- `GET /v1/me/memberships`

Behavior:

- Offer optional Tenderpreneur MFA after a recent sign-in.
- Show recovery codes once and refresh the rotated session only after acknowledgement.
- Require explicit organization selection from the current membership directory.
- Enter `/tenderpreneur` only when canonical `/v1/me` resolves it.

## Resumption and storage

- Authentication cookies remain host-only and secure.
- CSRF, setup secrets, recovery codes and organization selection remain memory-only.
- No onboarding state is stored in `localStorage` or `sessionStorage`.
- A resumable wizard requires a server-owned onboarding-state contract; until then each page derives progress from `/v1/me`, verification status and current memberships.

## Implementation sequence

1. Accept the irreversible Tenderpreneur account-type confirmation UX.
2. Implement account-type selection as its own bounded operation.
3. Complete invitation acceptance in the browser using the existing contract.
4. Keep the implemented organization-request UI and BFF aligned to the approved unmounted contract.
5. Design and approve provisioning checkpoint transitions before implementing any resource creation.
6. Compose the wizard from backend-authoritative step resolution.
7. Perform mobile, keyboard, screen-reader and live backend verification.
