# TenderOS — Glossary, Assumptions, and Decision Register

## 1. Glossary

| Term | Definition |
|---|---|
| TenderOS | Working name for the complete SaaS product |
| Platform Admin | SaaS operator portal and privileged platform role family |
| Procurement Team | Buying organization and its procurement users |
| Tenderpreneur | Supplier, contractor, consultant, company, or bid team pursuing tenders |
| Tenant | An isolated organization workspace with its own database and storage boundary |
| Control plane | Platform-wide identities, tenant registry, subscriptions, operations, and configuration |
| Tenant data plane | Private organization database and storage containing business records |
| Exchange plane | Controlled public/cross-tenant projections and sealed transaction metadata |
| Platform tender | Tender authored and published by a Procurement tenant in TenderOS |
| External tender | Tender sourced outside TenderOS and imported by a Tenderpreneur |
| Public catalogue | Searchable approved platform tender projections and lawful source records |
| Deep Search | Multi-source, provenance-aware tender discovery, matching, monitoring, and research |
| Opportunity | A discovered or imported tender candidate before/while becoming a bid workspace |
| Bid workspace | Private Tenderpreneur work area for assessment, requirements, response, approvals, and submission |
| Compliance matrix | Requirement-by-requirement status, owner, evidence, verification, and risk record |
| Compliance vault | Reusable company documents, facts, staff, projects, certifications, and knowledge |
| Publication version | Immutable approved version of a tender exposed to participants |
| Addendum | Authorized published change to a tender or its instructions |
| Submission package | Final approved response version and manifest prepared for submission |
| Sealed submission | Encrypted platform submission unavailable to procurement users before authorized opening |
| Opening session | Governed process releasing sealed bids to authorized committee/evaluation users |
| Preliminary evaluation | Mandatory/eligibility/completeness stage |
| Technical evaluation | Scored or pass/fail assessment of technical criteria |
| Financial evaluation | Price and commercial assessment according to published rules |
| Human in the loop | Required authorized human review/decision before material action |
| Source citation | Exact link from AI/extracted claim to source document/version/location |
| Outbox | Transactional table recording events to publish after domain commit |
| Inbox | Consumer record preventing duplicate event side effects |
| Country pack | Configurable terminology, locale, currency, methods, document types, and policy defaults |
| Evidence register | Traceable proof that a requirement or release gate was verified |

## 2. Baseline assumptions

- The first commercial market is Kenya.
- Both private and public/regulated procurement contexts may use the product, subject to configuration and legal review.
- Tenderpreneurs need independent value from external tenders before a large buyer network exists.
- Each tenant must have its own PostgreSQL database.
- A user may belong to multiple tenants.
- Platform Admin is separate from tenant organization administration.
- The downloaded frontend template will be available before frontend implementation and has a valid commercial license.
- Go is the only primary transactional backend language; Python is limited to AI/search/document processing services.
- PostgreSQL is the transactional source of truth.
- Direct external portal submission is only enabled through official supported integration and human authorization.
- AI is advisory and source-grounded.
- Mobile web responsiveness is required; a native mobile app is not initial scope.
- Contract management after award begins as a handover/export, not a full CLM module.

## 3. Open product decisions

| ID | Decision needed | Default until decided |
|---|---|---|
| DEC-P-001 | Final brand/product name | Use TenderOS as working name |
| DEC-P-002 | Can one tenant be both buyer and bidder? | Separate tenant type/workspace; hybrid only after policy design |
| DEC-P-003 | Tenderpreneur verification/badging model | No public “verified” badge until verification governance exists |
| DEC-P-004 | Free trial and plan pricing | Entitlement engine supports; commercial values deferred |
| DEC-P-005 | Whether public visitors can submit clarification questions | Require authenticated participation initially |
| DEC-P-006 | Native mobile application | Responsive web only for MVP |

## 4. Open architecture decisions

| ADR | Decision | Required evidence |
|---|---|---|
| ADR-001 | Final monorepo and module layout | Build/test proof and team agreement |
| ADR-002 | Database cluster provider/topology | Cost, HA, backup, PgBouncer, region |
| ADR-003 | Tenant database credentials/pool model | Isolation, connection scale, rotation test |
| ADR-004 | Identity provider vs application-owned auth details | Security, SSO, MFA, cost, migration |
| ADR-005 | Sealed bid cryptography and key release | Threat model, KMS prototype, independent review |
| ADR-006 | Message broker | Operations skill, durability, load, local parity |
| ADR-007 | Search store and scale trigger | Relevance/load benchmark |
| ADR-008 | AI provider/model routing | Quality, cost, privacy, region, fallback |
| ADR-009 | OCR/parser stack | Benchmark on real tender corpus |
| ADR-010 | Payment provider(s) | Kenya support, recurring billing, webhooks, reconciliation |
| ADR-011 | Production orchestration | Team capability, scale, reliability, cost |
| ADR-012 | Analytics store | Volume, privacy, reporting latency |

## 5. Open governance decisions

- Procurement workflow templates and separation-of-duty defaults
- Opening quorum and emergency procedure defaults
- Retention periods by record type and tenant type
- Whether/when bid prices may appear in opening minutes/public data
- Supplier/Tenderpreneur identity disclosure rules for clarifications
- Country-specific disqualification and appeal/review workflows
- Platform support access approval model
- AI provider data residency/retention contract
- Source crawling terms and connector approval process
- Record/e-signature requirements for legally sensitive procedures

## 6. Decision record template

```markdown
# ADR/Decision <ID>: <Title>

- Status: Proposed | Accepted | Superseded | Rejected
- Date:
- Owners:
- Related requirements:

## Context

## Options considered

## Decision

## Consequences

## Security/privacy/compliance impact

## Migration/rollout

## Validation evidence

## Supersedes / superseded by
```

## 7. Assumption validation rule

An assumption becomes a decision or requirement when implementation depends on it. The responsible ticket must either:

- Validate it with evidence,
- Record an accepted decision, or
- Mark the work blocked/deferred.

Agents and developers must not silently convert assumptions into permanent architecture.

