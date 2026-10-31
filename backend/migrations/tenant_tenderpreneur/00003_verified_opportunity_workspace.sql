-- +goose Up
-- +goose StatementBegin
-- BID-BE-002 approved simple human verification and one-workspace policy.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE bidora_opportunity.source_versions
    DROP CONSTRAINT source_versions_verification_state_check,
    ADD COLUMN verified_by uuid,
    ADD COLUMN verified_at timestamptz,
    ADD CONSTRAINT source_versions_verification_state_check
        CHECK (verification_state IN ('UNVERIFIED','VERIFIED')),
    ADD CONSTRAINT source_versions_verifier_check CHECK (
        (verification_state = 'UNVERIFIED' AND verified_by IS NULL AND verified_at IS NULL)
        OR
        (verification_state = 'VERIFIED' AND verified_by IS NOT NULL AND verified_at IS NOT NULL)
    );

CREATE SCHEMA bid;
CREATE TABLE bid.workspaces (
    id uuid PRIMARY KEY,
    opportunity_id uuid NOT NULL REFERENCES bidora_opportunity.opportunities(id) ON DELETE RESTRICT,
    source_version_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    state text NOT NULL CHECK (state = 'DECISION_PENDING'),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    archived_at timestamptz,
    FOREIGN KEY (opportunity_id, source_version_id)
        REFERENCES bidora_opportunity.source_versions(opportunity_id, id) ON DELETE RESTRICT,
    CHECK (updated_at >= created_at),
    CHECK (archived_at IS NULL OR archived_at >= created_at)
);
CREATE UNIQUE INDEX workspaces_one_active_opportunity_idx
    ON bid.workspaces (opportunity_id)
    WHERE archived_at IS NULL;
CREATE INDEX workspaces_owner_state_idx
    ON bid.workspaces (owner_id, state, created_at DESC, id DESC)
    WHERE archived_at IS NULL;

DO $$
DECLARE affected integer;
BEGIN
    UPDATE bidora_meta.schema_compatibility
    SET schema_version = 3, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'tenant_tenderpreneur' AND schema_version = 2;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 2';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Destructive rollback is local/empty-database development convenience only.
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';
UPDATE bidora_meta.schema_compatibility
SET schema_version = 2, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'tenant_tenderpreneur' AND schema_version = 3;
DROP SCHEMA bid CASCADE;
ALTER TABLE bidora_opportunity.source_versions
    DROP CONSTRAINT source_versions_verifier_check,
    DROP CONSTRAINT source_versions_verification_state_check,
    DROP COLUMN verified_at,
    DROP COLUMN verified_by,
    ADD CONSTRAINT source_versions_verification_state_check CHECK (verification_state = 'UNVERIFIED');
-- +goose StatementEnd
