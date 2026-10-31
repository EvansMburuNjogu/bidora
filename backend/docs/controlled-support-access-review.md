# BID-AUTH-004 / BID-API-010S — S1–S13 controlled-support review

Status: **SELECTIVE POLICY APPROVAL RECORDED — SESSION/EVIDENCE/FENCING DECISIONS OPEN**  
Reviewed packet: `controlled-support-access-decision-packet.md`  
Scope: tenant approval, diagnostic scope, support session, evidence, revocation and emergency-access review only  
Implementation authority: none

## 1. Review conclusion

S1–S13 correctly deny standing Platform access, impersonation, mutation, private-content download, generic proxying and automatic stale-cell forwarding. The proposed exact-scope, tenant-approved, short-lived, ordinary-session-bound support model is consistent with Bidora's database-per-tenant and server-owned routing rules. The existing `SUPPORT_ADMINISTRATOR` v1 bundle already has request/use/revoke capabilities, but those capabilities are inert without this separately approved policy.

The packet is not yet ready for approval as one executable contract. Tenant support capabilities do not exist; platform-recorded cases lack a tenant-confirmation rule; target discovery is underspecified; several DTO fields and stable failure codes need owner/classification approval; case free text creates a control-plane confidentiality risk; and the claimed revocation/disclosure ordering cannot be guaranteed across control and tenant databases as written without a reviewed fencing protocol. Tenant-visible evidence also needs truthful “authorized/released” semantics rather than claiming that a browser received or a person viewed data.

No support case, request, grant, cookie, diagnostic DTO, tenant projection or access may be activated from this review. Phase 1 emergency/break-glass private access should remain absent.

## 2. Remaining tenant-approval decisions

### T1 — exact Tenantpreneur capability bundle

S4 and S12 use capabilities that are not in an approved tenant catalogue:

- `tenderpreneur.organization.support_case.manage` / `.read`;
- `tenderpreneur.organization.support_access.approve` / `.read`.

Choose who receives them. **Recommended:** add narrowly named create/read/approve/history capabilities only to the fixed effective Organization Administrator bundle. Keep deny precedence, current context generation, ACTIVE tenant/membership, recent TOTP for approval/revocation and final-effective-administrator continuity. These capabilities grant no diagnostic content access beyond the tenant's ordinary authority, no Platform role, no hold/privacy authority and no support session.

Avoid a broad `.manage` capability if create, read, close and approve can be named separately. Exact capability names must be frozen before an API or migration.

### T2 — tenant-opened versus platform-recorded cases

A Platform Support Administrator may need to record phone/email contact, but an asserted contact is not tenant authority.

Choose one:

1. **Recommended:** a platform-recorded case remains `WAITING_FOR_TENANT_CONFIRMATION`; an effective Organization Administrator must adopt/confirm the exact case before any access request can be created. Recording creates no access, approval or proof that the contact was authorized.
2. Allow an access request immediately after a support actor records “authenticated tenant contact.” This requires an exact external authentication protocol and evidence policy, which does not exist.
3. Disallow platform-recorded cases in Phase 1. This is safest but may exclude ordinary support channels.

A tenant-opened case already supplies ordinary current tenant authority but still does not approve support access.

### T3 — one tenant approver versus tenant maker/checker

Decide whether one effective Organization Administrator with recent TOTP is sufficient. **Recommended:** one tenant approver is sufficient for the read-only S2 catalogue because the Platform requester is a distinct actor, scopes are content-minimized, duration is at most 60 minutes and every disclosure is tenant-visible. The approver may only narrow/reject. Any future content, export, download or mutation scope requires a new maker/checker policy.

If no current effective approver exists, access is denied. Platform Owner, Support, Security/Audit or deployment operators cannot approve for the tenant.

### T4 — conflict-of-interest rule

S4 refers to an identity “linked to the same person under a separately discovered conflict rule,” but no cross-identity person-linking authority or dataset exists. Do not claim detection that cannot be proved.

**Recommended:** enforce only authoritative facts Bidora possesses: the support actor must be a current `PLATFORM_ADMIN`; platform-identity activation already excludes tenant membership/pending invitation; every request rechecks no affected-tenant membership, pending invitation or active support-session conflict. Any employment/contractual conflict attestation remains outside application authority unless a separately approved conflict registry is introduced.

### T5 — continuing approver authority

S7/S9 propose that approver capability loss invalidates the session. Decide whether approval survives the approver's later offboarding. **Recommended:** recheck the exact approving actor's current verified/enabled membership and effective approval capability on every action. Loss immediately invalidates the grant; another administrator cannot inherit it and must approve a new request. This is stricter than treating approval as irrevocable historical consent.

### T6 — case close authority

Define who may resolve/close and reopen cases. **Recommended:** tenant case creators/effective Organization Administrators and the assigned Support Administrator may propose resolution; either side may close, but closure immediately invalidates pending/active access. Closed cases are immutable and cannot reopen; continuing work uses a linked new case. Support cannot close a case to erase tenant-visible history.

## 3. Remaining diagnostic-scope decisions

### D1 — case-text classification and storage

S1 permits 2,000 bytes of tenant-confidential description in what is otherwise a control-plane case. Warnings do not prevent users from pasting tender, personal or credential material.

Choose one:

1. **Recommended:** control stores category, bounded subject, safe structured symptom codes and a content-minimized tenant-authored summary explicitly shared for support; apply sanitization, classification, restricted case-participant access and an approved retention class. Never copy it to audit/outbox/notifications. Reject attachments, URLs and secret-like structured fields, while acknowledging that free-text secret detection is not proof.
2. Store free-form detail tenant-locally and expose it only through a separately approved case-message disclosure scope. This preserves plane separation but adds cross-plane complexity before access approval.
3. Keep unrestricted free text in control. This conflicts with minimization and should be rejected.

Exact subject/summary visibility after case assignment and retention remains a privacy/support policy decision.

### D2 — exact resource target nomination

Support cannot request an exact private bid/document/AI UUID unless it receives that identifier through an authorized path. It must not gain broad search merely to discover targets.

**Recommended:** tenant case creation/confirmation nominates exact resource IDs through an ordinary tenant-authorized picker, or the tenant supplies a server-issued opaque diagnostic reference. Support may request only nominated targets. Tenant-level DTOs remain limited to the explicit tenant scopes. No title/filename/email lookup, arbitrary UUID probing or list/search fallback is allowed.

### D3 — membership-specific diagnostics

`MEMBERSHIP_SUMMARY_READ` currently permits an optional exact membership UUID status. That becomes person-specific even without email.

Choose one:

1. **Recommended:** Phase 1 returns aggregate state/policy-health counts only. Exact membership diagnosis remains an ordinary tenant-admin operation.
2. Permit exact status only when the tenant explicitly nominates that membership target and privacy review approves the minimal fields.

Support never receives email, profile, grants or invitation token facts.

### D4 — owner-approved DTO and failure-code registry

Each S2 DTO needs an immutable schema/policy version owned by its module. Owners must approve every field, classification, stable failure code and corruption behavior. “Stable failure code” cannot mean raw scanner/provider/database diagnostics. Unknown fields/codes fail closed and never pass through.

Additive DTO fields are not automatically within an old approval: a material field/classification change creates a new support policy version and requires fresh tenant approval for new sessions.

### D5 — scope limits and combinations

Approve or amend one-to-five scope entries, twenty total exact targets and the tenant-wide DTO list. **Recommended:** retain those ceilings and reject duplicate/equivalent scopes canonically. Do not allow a combination of individually safe DTOs to reconstruct excluded private content or personal profiles; classification review must assess the combined response.

### D6 — subscription and control-plane diagnostics

`TENANT_CONFIGURATION_READ` and `SUBSCRIPTION_USAGE_DIAGNOSTIC_READ` originate partly in the global control plane, not the tenant database. Decide whether tenant-local disclosure evidence is still mandatory. **Recommended:** yes—tenant approval and tenant-visible evidence remain mandatory even when the redacted DTO source is control-plane data. The owner transaction/evidence sequence must not pretend a tenant SQL transaction atomically covers a control read.

## 4. Remaining support-session decisions

### SS1 — support-cookie scope

**Correction recorded in `controlled-support-session-evidence-fencing-decision.md`:** a conforming `__Host-` cookie must use `Path=/`; it cannot use a narrow `/v1/support-diagnostics` path. **Recommended:** retain `__Host-bidora_support` with Secure, HttpOnly, SameSite=Strict, `Path=/` and no Domain. Only exact support-diagnostic middleware may consume it, and unrelated handlers must neither interpret nor log it. Cookie path is not an authorization boundary; exact server-side session binding and per-action checks remain mandatory.

### SS2 — session concurrency

Decide whether one active support session per Support Administrator globally is acceptable. **Recommended:** retain one active session globally and one per approved request for Phase 1. It reduces tenant-context confusion and token mix-up. A support actor must end the current session before activating another tenant's grant; there is no tab-local or multi-tenant exception.

### SS3 — idle-time touch semantics

Define when successful activity extends the ten-minute idle boundary. **Recommended:** only an authorized diagnostic operation that reaches the tenant/control owner and commits its pre-disclosure evidence may advance last activity. Authentication failures, concealed-not-found, dependency failures, blocked final checks and ordinary case/session reads do not extend it. The touch uses database time and cannot exceed absolute/grant expiry.

The exact cross-plane touch/finalization protocol remains open under R1.

### SS4 — activation and ordinary-session changes

Any ordinary Platform session rotation, revocation, auth-epoch/context change, MFA loss or Support role/capability loss invalidates the bound support session immediately in authorization terms. A replacement Platform session never adopts the support grant; a fresh activation is prohibited once the 15-minute activation window closes and otherwise still requires the exact named actor and unused approved grant.

### SS5 — no asynchronous authority

The support session cannot enqueue a job, export, download, model call, notification containing private data or delayed read. Every permitted action is a synchronous bounded diagnostic read with current checks and evidence. A returned DTO cannot become a reusable credential.

### SS6 — raw token handling

Define redaction at middleware, panic/error and telemetry boundaries. The support cookie is accepted only from its exact cookie name; never from query, body, generic Authorization header or logs. Hash domain/version is explicit, raw bytes are returned once, and malformed/missing cookies fail generically without tenant lookup.

## 5. Remaining evidence decisions

### E1 — truthful disclosure states

A server cannot prove that a browser rendered or a person viewed bytes. Evidence should distinguish:

- `READ_PREPARED` — owner transaction read the DTO and committed tenant-local evidence;
- `RELEASE_AUTHORIZED` — the final current authorization check permitted response release;
- `RELEASE_DENIED` — a final check prevented returning DTO bytes;
- transport completion, if observed, as non-authoritative operational metadata only.

Do not label an event `VIEWED`, `RECEIVED` or `DELIVERED` based only on an HTTP write.

### E2 — control/tenant sequencing

S8 correctly requires tenant-local evidence before returning tenant-derived DTO bytes, but a control receipt after tenant commit is not atomic. Define a durable correlation/finalization protocol:

1. control authorizes one exact action attempt against current grant/session;
2. owner plane commits `READ_PREPARED` plus exact DTO snapshot/version or obtains the DTO in the same bounded transaction;
3. control performs a final current-session/grant/revocation check and records `RELEASE_AUTHORIZED` or denial;
4. only an authorized result is serialized;
5. reconciliation links tenant preparation and control outcome without rewriting either.

No SQL transaction is held across control/tenant calls. Crash or ambiguity may leave a prepared-but-not-released event, which is evidence—not a disclosure claim.

### E3 — evidence visibility and personal data

Approve the tenant-visible actor representation. **Recommended:** show stable support actor display name/role snapshot and opaque actor ID necessary for accountability, but no email, session/device/IP or unrelated profile. Historical display is immutable evidence and follows an approved support-evidence retention class.

Purpose text is tenant-approved case material, not general audit/outbox content. General events carry only purpose hash and safe codes.

### E4 — evidence retention and export

Support case, request, approval, session and action evidence needs an approved retention class/lawful purpose under pending R1–R13/privacy policy. Tenant-visible listing is not a bulk export and must be bounded/keyset paginated. No support evidence export exists in S1–S13.

### E5 — failed/denied attempts

Decide what tenant users see for unauthorized probes and concealed-not-found outcomes. **Recommended:** tenant evidence records attempts only after a valid active session/tenant scope is established; pre-session malformed/forged probes stay in restricted platform security telemetry to avoid creating attacker-controlled tenant records. Valid-session out-of-scope attempts append safe tenant-visible denial evidence without disclosing the probed resource.

## 6. Remaining revocation decisions

### R1 — cross-plane revocation/disclosure race

S9 claims that a revocation committed before the tenant disclosure transaction's final check prevents disclosure, but tenant SQL cannot atomically observe a concurrent control-plane commit. A simple replicated grant projection has a propagation window.

Choose a reviewed fencing protocol. Options include:

1. **Recommended:** control owns a short-lived single-action permit bound to session/grant/version/action/target; owner plane records the exact permit in `READ_PREPARED`; control performs the final authoritative revocation/version check and consumes/finalizes the permit immediately before release. Revocation invalidates every unfinalized permit. The system guarantees denial when revocation is observed before final permit consumption, not impossible real-time recall after release authorization.
2. Require the tenant transaction to synchronously call control while open. This violates the no-external-call-in-SQL-transaction rule and is rejected.
3. Rely only on asynchronous projection. This cannot support the packet's immediate-race claim and must disclose bounded staleness, which conflicts with “no cache grace.”

Exact permit lifetime, concurrency and crash reconciliation require a later implementation decision and tests.

### R2 — authoritative revokers

Approve the proposed revokers:

- any current effective tenant support approver for the exact tenant;
- the support actor ending their own session;
- current Support or Security/Audit Administrator with `platform.support.session.revoke`;
- automatic authoritative invalidation events.

Platform Owner has no implicit revocation capability unless separately assigned a role that carries it. Revokers cannot use support-session authority to revoke or alter history.

### R3 — case closure and approval loss

Case closure, tenant approver loss and tenant suspend/close should atomically invalidate control eligibility and emit a generation/version change. Tenant projections reconcile that fact, while every new action also requires the current control permit under R1. Pending requests expire/cancel; active sessions become unusable. No automatic transfer to another approver or case occurs.

### R4 — tenant movement and dependency loss

Assignment-generation change immediately denies new permits. The old cell receives no new read and the request/session is never forwarded to the replacement cell. A continuing investigation requires a new case/request/approval after movement reconciliation. Control or tenant dependency uncertainty denies release; it does not extend expiry or consume a retry that fabricates evidence.

### R5 — revocation notification and evidence

Tenant and support views should reflect authoritative revocation immediately after control commit, with projection state separately shown as reconciling where needed. Notifications are generic best effort and not authority. Revocation completion must not claim every projection reconciled until exact acknowledgements exist.

## 7. Emergency-access decision

### B1 — no Phase 1 break glass

**Recommended:** approve S11 unchanged. If no effective tenant approver is available, tenant-private support access is denied. Incident responders may use separately approved non-disclosing containment operations—identity/session revocation, integration pause or tenant suspension—but cannot inspect tenant-private data.

There is no environment flag, database credential, Platform Owner override, support self-approval or operational runbook workaround. Any future break glass requires a new threat model, two-person accountable approval, exact exclusions, shorter duration, customer notice and independent review; this packet provides no latent implementation hook.

## 8. Decisions that can be accepted independently

The following principles can be approved before the fencing/evidence protocol is implemented:

- content-minimized cases create no access;
- only exact read-only owner-approved diagnostic DTOs are eligible;
- Support Administrator request and tenant capability approval are distinct;
- tenant approver can only narrow/reject;
- 24-hour request, 15-minute activation, 15/30/60-minute grant and ten-minute idle ceilings with no renewal;
- support token is hash-only and bound to one ordinary Platform session;
- no tenant header/database locator, generic proxy, search, download, export, AI/provider call, asynchronous job, impersonation or mutation;
- stale assignment and uncertain dependency fail closed;
- tenant-visible evidence precedes release and never claims human viewing;
- history is append-only and revocation cannot be bypassed by cache;
- no Phase 1 emergency/break-glass access.

Approval of principles does not authorize a route, cookie or tenant read.

## 9. Selective approval record and remaining sequence

The requesting user approved these policy choices:

- **T1:** narrow case create/read/close and access approve/read/history semantics belong only to the fixed effective Organization Administrator bundle; broad `.manage` authority is rejected, while exact final capability identifiers remain a contract decision;
- **T2 option 1:** a platform-recorded case stays `WAITING_FOR_TENANT_CONFIRMATION` and cannot support an access request until an effective Organization Administrator adopts the exact case;
- **T3:** one effective Organization Administrator with recent TOTP may approve the read-only catalogue and may only narrow/reject; future content/export/download/mutation scopes require new policy;
- **T4:** enforce system-known Platform-versus-tenant membership/invitation/support conflicts only; do not claim unimplemented cross-identity person detection;
- **T5:** every action rechecks the original approver's current authority; loss invalidates the grant and another administrator must approve a new request;
- **T6:** tenant creator/effective Organization Administrator or assigned Support Administrator may propose resolution; either side may close, closure invalidates access, closed cases never reopen and continuing work uses a linked new case;
- **D1 option 1:** control stores only category, bounded subject, structured symptoms and an explicitly shared minimized summary under restricted access/retention; no attachments, URLs, secrets or audit/outbox copies;
- **D2:** tenants nominate exact private-resource targets through ordinary authority or a server-issued opaque diagnostic reference; Support gets no discovery search;
- **D3 option 1:** membership diagnostics are aggregate-only in Phase 1;
- **D4:** every diagnostic DTO and stable failure code is module-owned and versioned; material changes require a new support policy/approval;
- **D5:** retain one-to-five scope entries and twenty exact targets, canonical duplicate rejection and combined-classification review;
- **D6:** tenant approval and tenant-visible evidence remain mandatory for control-plane-origin subscription/configuration diagnostics, without pretending one tenant SQL transaction covers the control read;
- **B1:** Phase 1 has no break glass, Platform Owner override, support self-approval, environment bypass or database workaround.

The user explicitly directed **do not implement yet**. Exact tenant capability identifiers and every session/evidence/fencing decision remain unresolved. SS1–SS6, E1–E5 and R1–R5 therefore remain pending; no token/cookie, support session, DTO disclosure, evidence finalization or revocation behavior is executable.

Remaining sequence:

1. Decide the separate SS1–SS6, E1–E5 and R1–R5 ballot in `controlled-support-session-evidence-fencing-decision.md`, including its explicit numeric session baseline.
2. Freeze exact tenant capability names and amend S1–S13 with the complete session/evidence/revocation contract.
3. Only after a new explicit request consider separately bounded non-runtime contract/source implementation.

No capability, role/grant bundle, case, schema, migration, OpenAPI, route, token/cookie, support session, tenant projection, diagnostic DTO, owner read, evidence, worker, runtime, test or frontend change was made or authorized by this review.
