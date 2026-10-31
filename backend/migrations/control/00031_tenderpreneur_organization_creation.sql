-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';

CREATE TABLE tenancy.organization_creation_operations(
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL UNIQUE REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 creator_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 creator_membership_id uuid NOT NULL UNIQUE REFERENCES tenancy.memberships(id) ON DELETE RESTRICT,
 commercial_path text NOT NULL CHECK(commercial_path IN('TRIAL','PAID')),
 business_type text NOT NULL CHECK(length(business_type) BETWEEN 1 AND 100 AND business_type=trim(business_type)),
 primary_industry text NOT NULL CHECK(length(primary_industry) BETWEEN 1 AND 160 AND primary_industry=trim(primary_industry)),
 preferred_language text NOT NULL CHECK(length(preferred_language) BETWEEN 1 AND 35 AND preferred_language ~ '^[A-Za-z0-9-]+$'),
 currency char(3) NOT NULL CHECK(currency='KES'),
 state text NOT NULL CHECK(state IN('REQUESTED','PLAN_SELECTED','PROVISIONING','SUBSCRIPTION_ACTIVATION','ACTIVE','FAILED','REVIEW_REQUIRED','CANCELLED')),
 version bigint NOT NULL CHECK(version>0),
 correlation_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL CHECK(updated_at>=created_at)
);
CREATE UNIQUE INDEX organization_creation_one_open_per_creator_idx ON tenancy.organization_creation_operations(creator_user_id) WHERE state IN('REQUESTED','PLAN_SELECTED','PROVISIONING','SUBSCRIPTION_ACTIVATION','REVIEW_REQUIRED');

CREATE TABLE tenancy.organization_creation_lifetime_markers(
 creator_user_id uuid PRIMARY KEY REFERENCES identity.users(id) ON DELETE RESTRICT,
 operation_id uuid NOT NULL UNIQUE REFERENCES tenancy.organization_creation_operations(id) ON DELETE RESTRICT,
 tenant_id uuid NOT NULL UNIQUE REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 reason text NOT NULL CHECK(reason IN('NON_REVERSIBLE_RESOURCE','ACTIVE')),
 marked_at timestamptz NOT NULL
);

CREATE TABLE tenancy.provisioning_jobs(
 id uuid PRIMARY KEY,
 operation_id uuid NOT NULL UNIQUE REFERENCES tenancy.organization_creation_operations(id) ON DELETE RESTRICT,
 tenant_id uuid NOT NULL UNIQUE REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 state text NOT NULL CHECK(state IN('WAITING_PAYMENT','PENDING','RUNNING','RETRYABLE','REVIEW_REQUIRED','COMPLETED','CANCELLED')),
 checkpoint text NOT NULL CHECK(checkpoint IN('PLAN_SELECTED','PAYMENT_RECONCILED','DATABASE_RESERVED','DATABASE_CREATED','MIGRATIONS_APPLIED','RUNTIME_GRANTS_APPLIED','COMPATIBILITY_VERIFIED','ASSIGNMENT_ACTIVATED','SUBSCRIPTION_ACTIVATED','ACTIVE')),
 attempts integer NOT NULL CHECK(attempts BETWEEN 0 AND 3),
 lease_until timestamptz,
 next_attempt_at timestamptz,
 safe_failure_code text CHECK(safe_failure_code IS NULL OR safe_failure_code IN('CAPACITY_UNAVAILABLE','DATABASE_RESERVATION_FAILED','DATABASE_CREATION_FAILED','MIGRATION_FAILED','RUNTIME_GRANT_FAILED','COMPATIBILITY_FAILED','ASSIGNMENT_FAILED','SUBSCRIPTION_ACTIVATION_FAILED','EFFECT_AMBIGUOUS','INTERNAL_ERROR')),
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL CHECK(updated_at>=created_at),
 CHECK((state='RUNNING')=(lease_until IS NOT NULL)),
 CHECK((state IN('RETRYABLE','REVIEW_REQUIRED')) OR safe_failure_code IS NULL)
);
CREATE INDEX provisioning_jobs_ready_idx ON tenancy.provisioning_jobs(state,next_attempt_at,id) WHERE state IN('PENDING','RETRYABLE');

CREATE TABLE tenancy.organization_creation_commands(
 id uuid PRIMARY KEY,
 creator_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 operation_id uuid NOT NULL REFERENCES tenancy.organization_creation_operations(id) ON DELETE RESTRICT,
 tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 response_state text NOT NULL CHECK(response_state='REQUESTED'),
 created_at timestamptz NOT NULL,
 completed_at timestamptz NOT NULL CHECK(completed_at>=created_at),
 UNIQUE(creator_user_id,key_hash)
);

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=31,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=30;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 30'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM tenancy.organization_creation_operations) THEN RAISE EXCEPTION 'organization creation history prevents downgrade'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=30,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=31;
DROP TABLE tenancy.organization_creation_commands;
DROP TABLE tenancy.provisioning_jobs;
DROP TABLE tenancy.organization_creation_lifetime_markers;
DROP TABLE tenancy.organization_creation_operations;
-- +goose StatementEnd
