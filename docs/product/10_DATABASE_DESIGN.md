# TenderOS — Database and Data Architecture

## 1. Database topology

TenderOS uses three database categories:

1. **Control database:** one platform database for identities, tenant registry, subscriptions, source registry, and platform operations.
2. **Tenant databases:** one private PostgreSQL database per Procurement or Tenderpreneur tenant.
3. **Exchange database:** one logical platform database for public tender projections, sealed-submission metadata, and cross-tenant exchange state.

Large enterprise tenants may receive dedicated clusters. The logical contract remains database-per-tenant.

## 2. Data isolation rules

- Tenant business tables exist only in the tenant's database.
- No cross-tenant foreign key, SQL join, or database link is permitted.
- Database selection is derived from authenticated context and the tenant registry.
- Each database has tenant-specific application credentials or securely isolated role mapping.
- Object storage, search indexes, embeddings, backups, and exports use the same isolation principle.
- The exchange database holds only approved public data or minimum cross-tenant transaction metadata.

## 3. Identifier and common-column standards

Recommended identifiers: application-generated UUIDv7 or equivalent time-sortable UUID.

Common columns:

```sql
id                uuid primary key,
status            text not null,
version           bigint not null default 1,
created_at        timestamptz not null,
created_by        uuid,
updated_at        timestamptz not null,
updated_by        uuid,
deleted_at        timestamptz,
deleted_by        uuid
```

Not every append-only table needs `updated_*` or soft deletion. Domain lifecycle fields must be explicit.

Standards:

- Money: `numeric(20,4)` plus ISO currency code.
- Percent/score: constrained numeric, not float.
- Time: `timestamptz`; retain source timezone/name separately for deadlines.
- JSONB: only for flexible metadata, event payloads, or versioned configuration—not as a substitute for core relational design.
- Enumerations: lookup/check constraints or versioned code tables; avoid database enum types when frequent evolution is expected.
- Text search: generated tsvector/index where appropriate.

## 4. Control database schemas and tables

### 4.1 `identity`

- `users`
- `user_emails`
- `password_credentials`
- `external_identities`
- `mfa_methods`
- `sessions`
- `session_events`
- `recovery_challenges`
- `security_risk_events`

### 4.2 `tenancy`

- `tenants`
- `tenant_types`
- `tenant_database_assignments`
- `database_clusters`
- `tenant_storage_assignments`
- `tenant_schema_versions`
- `tenant_provisioning_jobs`
- `tenant_lifecycle_events`
- `memberships`
- `membership_role_grants`
- `invitations`

`tenant_database_assignments` contains a secret reference, never plaintext credentials.

### 4.3 `subscription`

- `plans`
- `plan_versions`
- `features`
- `plan_entitlements`
- `subscriptions`
- `subscription_events`
- `usage_meters`
- `usage_ledger`
- `usage_reservations`
- `invoices`
- `invoice_lines`
- `payments`
- `payment_webhook_events`

Usage ledger is append-oriented and aggregated to current usage projections.

### 4.4 `platform`

- `feature_flags`
- `country_packs`
- `reference_data_versions`
- `source_registry`
- `source_connectors`
- `connector_runs`
- `integration_credentials` (secret references)
- `platform_announcements`
- `support_access_requests`
- `support_access_sessions`
- `platform_audit_events`

## 5. Common tenant schema

Every tenant database contains common modules.

### 5.1 Organization and access projection

- `organization_profile`
- `departments`
- `teams`
- `tenant_user_profiles`
- `role_definitions`
- `role_assignments`
- `permission_overrides`
- `organization_settings`

The control plane is authoritative for identity/membership; tenant DB retains a local projection needed for business assignments and audit.

### 5.2 Document and knowledge

- `documents`
- `document_versions`
- `document_files`
- `document_access_policies`
- `document_verifications`
- `document_expiry_rules`
- `knowledge_items`
- `knowledge_item_versions`
- `knowledge_approvals`
- `document_usage_links`
- `retention_holds`

Important uniqueness/constraints:

- File checksum + logical document/version relationship
- One current approved version per governed knowledge item
- Expiry date after issue date where both exist
- Malware state must be `CLEAN` before `AVAILABLE`

### 5.3 Workflow and collaboration

- `workflow_templates`
- `workflow_template_versions`
- `workflow_steps`
- `workflow_instances`
- `workflow_step_instances`
- `approval_decisions`
- `delegations`
- `tasks`
- `task_dependencies`
- `comments`
- `mentions`
- `notification_preferences`
- `notification_outbox`
- `notification_deliveries`

### 5.4 Audit and eventing

- `audit_events`
- `evidence_manifests`
- `outbox_events`
- `inbox_events`
- `idempotency_records`
- `scheduled_jobs`

Audit events are append-only. Update/delete permissions are denied to the application role.

### 5.5 AI and search

- `ai_jobs`
- `ai_job_inputs`
- `ai_job_outputs`
- `ai_citations`
- `ai_user_dispositions`
- `ai_usage_records`
- `document_chunks`
- `chunk_embeddings` or pgvector column
- `search_index_state`

Private chunks and embeddings remain in the tenant database or a tenant-isolated index namespace.

## 6. Procurement tenant schema

### 6.1 Planning

- `procurement_plans`
- `procurement_plan_items`
- `procurement_requests`
- `procurement_request_versions`
- `budget_confirmations`
- `method_selections`

### 6.2 Tender authoring

- `tenders`
- `tender_versions`
- `tender_lots`
- `tender_schedules`
- `tender_requirements`
- `evaluation_criteria`
- `criterion_scales`
- `submission_fields`
- `tender_forms`
- `tender_documents`
- `tender_review_findings`

Recommended constraints:

- Unique `(tender_reference, procurement_period)`
- Requirement/criterion sequence unique within version/lot/section
- Criterion weight and score ranges constrained
- Closing after opening/publication dates
- Published version reference immutable

### 6.3 Publication and clarification

- `publication_intents`
- `publication_deliveries`
- `tender_invitations`
- `clarification_questions`
- `clarification_responses`
- `addenda`
- `addendum_changes`
- `participant_acknowledgements`

### 6.4 Submission projection and opening

Procurement tenant stores metadata projections before opening and content references after release:

- `submission_receipts`
- `opening_sessions`
- `opening_committee_members`
- `conflict_declarations`
- `opening_release_records`
- `opening_minutes`

### 6.5 Evaluation

- `evaluation_cases`
- `evaluation_stages`
- `evaluator_assignments`
- `preliminary_findings`
- `criterion_scores`
- `evaluation_evidence_refs`
- `evaluation_clarifications`
- `arithmetic_corrections`
- `score_locks`
- `moderation_records`
- `evaluation_results`
- `evaluation_reports`

Use uniqueness on evaluator + submission + criterion + attempt/version.

### 6.6 Award

- `award_recommendations`
- `award_decisions`
- `award_approvals`
- `outcome_notices`
- `award_publications`
- `contract_handovers`

## 7. Tenderpreneur tenant schema

### 7.1 Organization capability profile

- `business_capabilities`
- `products_services`
- `certifications`
- `licences`
- `staff_profiles`
- `staff_qualifications`
- `past_projects`
- `project_references`
- `manufacturer_authorizations`
- `financial_profile_records`
- `search_preferences`

Sensitive fields use classification and field-level authorization.

### 7.2 Opportunities and external sources

- `opportunities`
- `opportunity_sources`
- `external_tender_imports`
- `extracted_tender_facts`
- `fact_verifications`
- `opportunity_matches`
- `saved_searches`
- `watchlists`
- `watchlist_targets`
- `opportunity_alerts`

### 7.3 Bid workspace

- `bid_workspaces`
- `bid_workspace_versions`
- `bid_decisions`
- `bid_assessments`
- `bid_assessment_factors`
- `bid_requirements`
- `requirement_assignments`
- `requirement_evidence`
- `response_sections`
- `response_section_versions`
- `financial_responses`
- `financial_response_lines`
- `bid_reviews`
- `bid_readiness_results`
- `bid_risks`
- `submission_packages`
- `external_submission_evidence`
- `bid_outcomes`
- `lessons_learned`

Constraints:

- Active workspace uniqueness by tenant + authoritative tender/lot unless variant flag
- Financial tables restricted through application policy and separate query adapters
- Approved response version immutable
- Evidence references include exact document version

## 8. Exchange database schema

### 8.1 Public catalogue

- `public_tenders`
- `public_tender_versions`
- `public_lots`
- `public_requirements`
- `public_criteria`
- `public_documents`
- `public_addenda`
- `public_clarifications`
- `public_source_links`

Public projection stores content approved for publication and source hashes.

### 8.2 Participation

- `tender_participation_registrations`
- `tender_followers`
- `invitation_deliveries`
- `addendum_acknowledgements`

### 8.3 Sealed submissions

- `submissions`
- `submission_versions`
- `submission_envelopes`
- `submission_manifest_entries`
- `submission_receipts`
- `submission_withdrawals`
- `submission_replacements`
- `opening_release_authorizations`
- `opening_release_events`

Do not store plaintext private submission content in ordinary columns.

### 8.4 Outcome exchange

- `award_outcomes`
- `outcome_delivery_receipts`
- `exchange_outbox_events`
- `exchange_inbox_events`

## 9. Deep Search platform data

Deep Search public-source data may use the exchange/search database or a dedicated search datastore.

Tables/index entities:

- `source_documents`
- `source_fetches`
- `canonical_opportunities`
- `opportunity_source_links`
- `source_content_versions`
- `source_change_events`
- `search_chunks`
- `search_embeddings`
- `deduplication_candidates`
- `source_health_metrics`

Required provenance:

- Source registry ID
- URL/identifier
- First and last seen
- Fetched/published/closing timestamps
- HTTP/content metadata
- Content hash and parser version
- Authoritative-source flag and confidence
- Original document reference

## 10. Indexing strategy

Examples:

- B-tree on state, date, owner, foreign keys, and frequently filtered columns
- Partial indexes for active/open/pending records
- GIN on `tsvector` search columns and selected JSONB metadata
- Vector index only after measuring dataset/query pattern
- Composite indexes aligned with tenant-database query patterns; tenant ID is not repeated in every tenant table unless needed for audit/import portability
- Exchange/control tables include tenant IDs and indexes because they are multi-tenant platform stores

Every index must be justified by a query or constraint; unnecessary indexes increase write and migration cost.

## 11. Migration strategy

### Migration sets

- Control
- Exchange
- Common tenant
- Procurement tenant extension
- Tenderpreneur tenant extension

### Rules

1. Migrations are immutable after release.
2. Each migration has forward SQL, verification query, data/backfill plan, compatibility impact, and recovery approach.
3. Prefer expand-and-contract for zero/low-downtime changes.
4. Application release must remain compatible with current and next schema version during rollout.
5. Tenant migrations run in canary batches and record per-tenant state.
6. Failed tenants are isolated and retried; rollout can pause.
7. Destructive changes require backup verification and explicit approval.

## 12. Provisioning workflow

```text
Create tenant registry record
→ choose cluster based on region/capacity/plan
→ create database and least-privileged roles
→ store credentials in secrets manager
→ apply common + tenant-type migrations
→ seed country/reference/workflow defaults
→ create object-storage namespace/policy
→ verify read/write/backup/health
→ mark tenant ACTIVE
```

Provisioning is idempotent and resumable.

## 13. Backup and restore

- Point-in-time recovery for production database clusters where supported
- Encrypted full/base backups and WAL/archive retention
- Tenant-level logical backup/export for portability and legal requests
- Object storage versioning and retention
- Control/exchange/tenant backup schedules coordinated but independently restorable
- Restore tests into isolated environments
- Evidence of backup age, success, checksum, and restore drill

Suggested starting targets, subject to plan/business approval:

- Control/exchange production RPO: 15 minutes or better
- Standard tenant RPO: 1 hour or better
- Critical enterprise tenant RPO: 15 minutes or contract-specific
- RTO differentiated by plan and incident severity

## 14. Retention and deletion

Retention policies are record-type and country/tenant configurable.

Lifecycle:

```text
ACTIVE → ARCHIVED → RETENTION_LOCKED/LEGAL_HOLD → ELIGIBLE_FOR_DELETION → DELETED/ANONYMIZED
```

Deletion must cover:

- Transactional rows
- Object versions
- Search indexes/embeddings
- Derived caches
- Exports
- Backups according to documented expiry

Audit/evidence that must be legally retained is separated from unnecessary personal content.

## 15. Data quality controls

- Reference codes and country-pack validation
- Required fields by state transition
- Referential and uniqueness constraints
- Source/fact verification status
- Currency and timezone explicitness
- Document issue/expiry validation
- Score/weight reconciliation
- Hash reconciliation for public projections and submissions
- Periodic orphan, stale projection, and index-rebuild checks

## 16. Reporting strategy

Operational queries use tenant database read models. Platform-wide analytics use privacy-minimized events/aggregates delivered to an analytics store; they do not query all tenant databases interactively.

Reports must define:

- Source tables/events
- Formula
- Timezone/currency treatment
- Freshness
- Permission/classification
- Reconciliation test

