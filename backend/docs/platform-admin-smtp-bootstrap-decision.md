# Initial Platform Administrator and SMTP bootstrap — decision requested

Status: **DECISION PENDING FOR INITIAL ADMINISTRATOR INVITATION DELIVERY AND SMTP ACTIVATION ONLY; NO BOOTSTRAP CHANGE AUTHORIZED**. Directory/detail reads, other administrator governance reads, tenant reads and unrelated Platform features proceed independently. This note is not a Platform Admin roadmap gate.

## Why the decision is needed

`platformadmin/domain/role.go` grants SMTP management to an Integration Administrator, not the initial Platform Owner. `platformsettings/application/service.go` requires a distinct effective Platform Owner checker for activation. `pilot/identity_composition.go` supplies Platform invitation delivery from the database-managed SMTP adapter, which cannot deliver until an active version exists; `pilot/runtime.go` mounts the invitation routes only when that delivery adapter is configured. Invitation tokens are generated once, hashed at rest and never returned for manual forwarding. The approved first-additional-owner invitation exception avoids a second checker **for that invitation**, not email delivery, SMTP configuration authority or SMTP maker/checker separation. The current local fixture has one effective owner and no active SMTP version; its checked activation remains outstanding. This creates a first-operator onboarding dependency; it is not permission to expose tokens, directly alter roles/settings tables or auto-activate a draft.

## Smallest policy choice to approve next

Choose **one** narrowly scoped bootstrap route after identity/security/deployment review:

1. **Temporary, explicit invitation-delivery bootstrap:** permit an authenticated, verified-trust STARTTLS sender for initial administrator invitations only, supplied by deployment configuration and retired once database-managed SMTP activates. Define credential custody, recipient eligibility, attempt evidence, expiry and fail-closed retirement. This requires a bounded adapter/composition contract; it must not become an ambient SMTP fallback.
2. **Governed additional-administrator bootstrap:** permit deployment-owned creation of the minimum eligible distinct Integration Administrator and second effective Platform Owner under an audited, single-use, time-bound authority contract. Define verified identity, TOTP enrollment, eligibility, separation of duties, rollback and evidence. This extends the current single-first-owner bootstrap policy; it cannot be inferred from it.

Do **not** select either option through application code or a fixture shortcut. After a choice, separately verify the first operator journey: authorised SMTP draft and synthetic test → distinct-owner activation → invitation delivery and acceptance → current directory/context/session authority. Provider acceptance and production secret handling remain separate gates. No database, host service, credential, role, contract or route was modified by this note.
