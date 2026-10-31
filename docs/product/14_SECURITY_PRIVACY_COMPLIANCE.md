# TenderOS — Security, Privacy, and Compliance Specification

## 1. Security objectives

TenderOS handles confidential procurement plans, supplier records, technical proposals, pricing, evaluation data, identities, and personal information. Security is a product requirement, not a deployment afterthought.

Objectives:

- Prevent cross-tenant disclosure or modification.
- Protect sealed submissions from premature access or tampering.
- Enforce least privilege and separation of duty.
- Preserve trustworthy audit and evidence.
- Protect personal and confidential data throughout its lifecycle.
- Resist malicious files, prompt injection, account takeover, API abuse, and insider misuse.
- Recover from failure, compromise, or data loss with tested procedures.

## 2. Governance baseline

The product must be configured and legally reviewed for each deployment context. For Kenya, the governance baseline should consider, at minimum:

- The Constitution's principles for public contracting where applicable
- Public Procurement and Asset Disposal Act, 2015 and applicable regulations/policies
- Kenya Data Protection Act, 2019 and applicable regulations/guidance
- Electronic transactions, evidence, records-retention, tax, sector, donor, and organizational requirements as applicable

TenderOS must not market itself as automatically guaranteeing legal compliance. It provides configurable controls and evidence; the customer remains responsible for approved procurement policy and lawful use.

## 3. Security roles and accountability

- Platform security owner
- Product/data protection owner
- Infrastructure/operations owner
- Tenant organization administrator
- Procurement security/governance owner
- Incident commander
- Security reviewer for sealed-submission design
- Data controller/processor contacts defined contractually

Privileged access is reviewed regularly. No shared administrator accounts are permitted.

## 4. Threat model

Priority threats:

1. Cross-tenant database, storage, search, or AI leakage
2. Broken object-level authorization/IDOR
3. Premature or unauthorized access to sealed bids
4. Submission tampering, replay, replacement, or timestamp dispute
5. Account takeover and session theft
6. Privilege escalation and separation-of-duty bypass
7. Malicious uploads, archives, macros, scripts, or polyglot files
8. Injection: SQL, command, template, HTML, spreadsheet, email, and log injection
9. SSRF through URL import or AI tools
10. Prompt injection and retrieval poisoning
11. Unauthorized model-provider data disclosure
12. Insider misuse by platform or tenant administrators
13. Source-connector abuse, scraping violations, or poisoned external data
14. DDoS, deadline-time submission spikes, and resource exhaustion
15. Secret exposure and supply-chain compromise
16. Audit deletion or manipulation
17. Backup exposure or failed recovery
18. Privacy overcollection and excessive retention

A formal threat model must be updated for each major release and before the sealed-submission production launch.

## 5. Authentication

Requirements:

- Verified email/identity before privileged workspace access
- Memory-hard password hashing when password login is enabled
- MFA support; mandatory for platform administrators and configurable high-risk tenant roles
- Session rotation after login, privilege change, MFA, password reset, and step-up
- Short bounded idle and absolute lifetimes according to risk
- Device/session list and revocation
- Login throttling, credential-stuffing detection, and risk alerts
- Recovery with single-use, expiring challenges and anti-enumeration responses
- Enterprise OIDC/SAML through approved adapters

Sensitive actions requiring step-up may include:

- Platform support access
- Tenant suspension/decommission
- Final bid submission
- Bid opening/key release
- Evaluation unlock
- Award approval
- Billing or bank/payment detail changes
- Bulk export
- Secret/key rotation

## 6. Authorization

Use default-deny RBAC + ABAC.

Authorization dimensions:

- Platform versus tenant boundary
- Tenant type
- Membership status
- Role and explicit grant/deny
- Department/team
- Resource ownership/assignment
- Tender, workflow, and evaluation stage
- Data classification
- Separation-of-duty policy
- Conflict-of-interest status
- Session assurance

Rules:

- Every API operation authorizes server-side.
- List/search queries are permission-filtered before results and counts are returned.
- Authorization failures do not reveal private record existence.
- Role administration enforces grantable-role boundaries and last-admin rules.
- Permission changes revoke or refresh affected sessions.
- AI and export jobs repeat authorization at execution time.

## 7. Tenant isolation

### Control requirements

- One private database per tenant
- Tenant DB chosen from authenticated active context
- Separate database roles/credentials and storage namespace
- No client-controlled database locator
- No cross-tenant joins or shared tenant tables for private business data
- Tenant-scoped cache keys and job queues
- Private search/embedding namespaces
- Tenant-aware logs and metrics without content exposure
- Automated isolation tests at API, repository, database router, object storage, search, events, exports, and AI layers

### Failure behavior

When tenant context is missing, stale, suspended, mismatched, or unavailable, fail closed. Never fall back to a default tenant database.

## 8. Database security

- TLS database connections
- Least-privileged roles: migrator, application read/write, read-only/reporting, backup
- Secrets in secrets manager, not environment files committed to source or control tables
- Network restrictions and private endpoints
- Bounded statement and transaction timeouts
- Parameterized SQL only
- Migration review and signed/reproducible artifacts
- Audit for privilege and schema changes
- Encryption at rest and encrypted backups
- Restore into isolated target before production cutover

Database-per-tenant does not eliminate application authorization; both are required.

## 9. Object and document security

- Quarantine all uploads
- Detect type from content; reject disallowed/ambiguous formats
- Size, archive depth, page, and decompression limits
- Malware scanning and optional content disarm/reconstruction
- Strip or block active content where appropriate
- Checksum every object/version
- Store using opaque server-generated keys
- Encrypt at rest; use tenant/exchange-specific keys or key context
- Short-lived signed URLs with exact object/action scope
- Prevent public bucket access except intentionally published tender artifacts
- Sanitize HTML/rich text/previews
- Protect against spreadsheet formula injection in exports
- Immutable version links for approved submissions/evidence

## 10. Sealed-bid security

Sealed submissions require controls beyond ordinary permissions.

### Required properties

- Confidentiality before opening
- Integrity and tamper evidence
- Authoritative server timestamp and acknowledgement
- Non-repudiation evidence appropriate to the legal/business context
- Controlled withdrawal/replacement history
- Key separation and release policy
- Opening quorum/role/time/conflict controls
- Complete release/opening audit

### Design requirements

- Encrypt submission package with a unique data-encryption key.
- Encrypt/wrap the data key using KMS/HSM-backed keys and opening policy.
- The ordinary procurement API runtime must not possess a standing pre-opening decryption path.
- Opening release requires authoritative tender state and approved committee conditions.
- Hash manifest and every file/structured response.
- Sign or otherwise protect receipt and opening evidence.
- Monitor and alert on attempted premature access or key operations.
- Subject cryptographic design, KMS policy, and incident procedures to independent review and penetration testing.

## 11. API and browser security

- TLS and HSTS
- Explicit production CORS allowlist
- Secure, HttpOnly, appropriate SameSite cookies
- CSRF controls for cookie-authenticated writes
- CSP, frame-ancestors, referrer policy, MIME sniffing protection, and permissions policy
- Input validation, canonicalization, and output encoding
- Rate limits by IP, identity, tenant, operation, and risk
- Request body/time limits
- Idempotency and replay protection for critical writes
- SSRF-safe URL fetcher with DNS/IP validation, redirect limits, protocol allowlist, egress policy, and size/time limits
- Safe file names and `Content-Disposition`
- No sensitive information in URLs

## 12. AI and model security

- Separate system policy from untrusted content.
- Treat documents/web pages as data, never instructions.
- Use allowlisted tools with job-scoped credentials.
- Authorize retrieval before model input.
- Apply data-classification rules to provider/model selection.
- Configure provider data retention/training controls contractually and technically.
- Redact/minimize personal and confidential data where not needed.
- Detect prompt injection, data-exfiltration attempts, and malicious links.
- No arbitrary code, shell, SQL, email, network, publication, submission, or award tools.
- Validate structured output and citations.
- Log model/job metadata without indiscriminate raw sensitive content.
- Red-team cross-tenant retrieval, prompt injection, tool abuse, and poisoned knowledge.

## 13. Privacy engineering

### Privacy principles

- Purpose limitation
- Data minimization
- Lawful and transparent processing
- Accuracy and user correction
- Access control and confidentiality
- Retention limitation
- Data-subject rights support
- Processor/subprocessor governance
- Privacy by design/default

### Data inventory and classification

Classify at least:

- Public tender content
- Internal business content
- Confidential procurement content
- Sealed bid content
- Commercially sensitive pricing
- Personal data
- Special/sensitive personal data where present
- Authentication secrets
- Audit/security data

### Required capabilities

- Privacy notice and consent/acknowledgement where appropriate
- Controller/processor configuration and contractual records
- Data export/access request workflow
- Correction and deletion/anonymization workflow subject to lawful retention
- Legal hold and retention schedule
- Breach assessment and notification workflow
- Subprocessor registry
- Cross-border processing and hosting assessment
- Data Protection Impact Assessment for high-risk features, especially AI, profiling, sealed bids, and large-scale supplier data

## 14. Audit and evidence integrity

Audit records include:

- Actor and effective role/context
- Time and authoritative server clock
- Action and target
- Previous/new state or change summary
- Reason where required
- IP/device/session metadata proportionate to need
- Correlation and causation IDs
- Outcome and error code
- Support-access or step-up reference

Audit requirements:

- Append-only application behavior
- Restricted database permissions
- Tamper-evident export/manifest for critical procedures
- Separate retention from diagnostic logs
- Search/export permissions
- Clock synchronization monitoring
- No secret or unnecessary document content in audit payloads

## 15. Logging and monitoring

- Structured logs with classification-aware redaction
- No passwords, access/refresh tokens, MFA secrets, full bid content, raw documents, or unnecessary personal data
- Detection for repeated authorization failures, tenant mismatch, premature bid access, unusual downloads/exports, role changes, connector abuse, AI exfiltration patterns, and payment abuse
- Central alerting and defined severity/ownership
- Security event retention aligned with investigation needs

## 16. Secure SDLC

Required pipeline controls:

- Code review and protected branches
- Static analysis and linting
- Dependency and vulnerability scanning
- Secret scanning
- Container/image scanning
- Infrastructure-as-code scanning
- SBOM generation
- Signed build artifacts/images where feasible
- License review
- Unit/integration/authorization/security tests
- DAST against staging
- Independent VAPT before major production milestones
- Dependency update policy and emergency patch process

No dependency is introduced solely because an AI agent generated it. New dependencies require purpose, owner, license, maintenance, and security review.

## 17. Incident response

Incident classes:

- Account compromise
- Cross-tenant exposure
- Sealed-bid confidentiality/integrity incident
- Malicious upload
- AI/data exfiltration
- Source-connector compromise
- Service outage/data loss
- Payment/billing incident
- Privacy breach

Runbook lifecycle:

```text
Detect → Triage → Contain → Preserve evidence → Eradicate
→ Recover → Notify/escalate as required → Review → Remediate
```

Critical actions such as tenant isolation, token revocation, connector pause, model-provider disablement, and key rotation must be operable without a full application release.

## 18. Business continuity and disaster recovery

- Document dependencies and failure modes
- Multi-zone/high-availability production design as justified
- Encrypted backups and point-in-time recovery
- Restore drills for control, exchange, and sampled tenant databases
- Object storage version recovery
- Message/outbox reconciliation after restore
- KMS/key recovery and escrow policy
- Defined RPO/RTO and crisis communications
- Manual fallback for AI/search outages
- Deadline-sensitive incident procedure for submissions and opening

## 19. Security acceptance gates

Before production:

- Threat model approved
- Tenant isolation suite passes
- Authorization matrix tests pass
- Sealed-bid design independently reviewed
- VAPT critical/high findings remediated or formally accepted
- Backup and restore drill passes
- Incident runbooks exercised
- Data inventory, retention, privacy notice, and processor agreements reviewed
- AI red-team and provider configuration reviewed
- Support access, export, and privileged role controls tested

## 20. Reference baseline

Authoritative legal and regulatory interpretation must come from qualified counsel and official Kenyan sources, including Kenya Law, the Public Procurement Regulatory Authority/National Treasury, and the Office of the Data Protection Commissioner. This specification intentionally avoids hardcoding legal section interpretations into product logic without review.

