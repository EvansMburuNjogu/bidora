-- +goose Up
-- +goose StatementBegin
-- BID-AUTH-003B / approved BID-API-010P P1-P6 persistence foundation.
-- Platform roles are fixed routing-independent authority facts and never grant
-- tenant membership, database selection or standing support access.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE SCHEMA platform_administration;

CREATE TABLE platform_administration.governance_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    bootstrap_consumed boolean NOT NULL DEFAULT false,
    bootstrap_owner_id uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
    bootstrap_consumed_at timestamptz,
    first_additional_owner_exception_consumed boolean NOT NULL DEFAULT false,
    effective_owner_generation bigint NOT NULL DEFAULT 0 CHECK (effective_owner_generation >= 0),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_at timestamptz NOT NULL,
    CHECK ((bootstrap_consumed AND bootstrap_owner_id IS NOT NULL AND bootstrap_consumed_at IS NOT NULL)
        OR (NOT bootstrap_consumed AND bootstrap_owner_id IS NULL AND bootstrap_consumed_at IS NULL))
);
INSERT INTO platform_administration.governance_state (updated_at) VALUES (CURRENT_TIMESTAMP);

CREATE TABLE platform_administration.administrator_profiles (
    user_id uuid PRIMARY KEY REFERENCES identity.users(id) ON DELETE RESTRICT,
    context_generation bigint NOT NULL DEFAULT 1 CHECK (context_generation > 0),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at)
);

CREATE TABLE platform_administration.role_assignments (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES platform_administration.administrator_profiles(user_id) ON DELETE RESTRICT,
    role_key text NOT NULL CHECK (role_key IN (
        'PLATFORM_OWNER','OPERATIONS_ADMINISTRATOR','BILLING_ADMINISTRATOR',
        'INTEGRATION_ADMINISTRATOR','SUPPORT_ADMINISTRATOR','SECURITY_AUDIT_ADMINISTRATOR'
    )),
    state text NOT NULL CHECK (state IN ('ACTIVE','REVOKED')),
    source text NOT NULL CHECK (source IN ('BOOTSTRAP','INVITATION','OWNER_CHANGE')),
    assigned_by uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
    assigned_at timestamptz NOT NULL,
    revoked_by uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
    revoked_at timestamptz,
    reason text NOT NULL CHECK (length(reason) BETWEEN 1 AND 2000 AND reason = btrim(reason)),
    revocation_reason text CHECK (revocation_reason IS NULL OR (length(revocation_reason) BETWEEN 1 AND 2000 AND revocation_reason = btrim(revocation_reason))),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK ((source = 'BOOTSTRAP' AND assigned_by IS NULL) OR (source <> 'BOOTSTRAP' AND assigned_by IS NOT NULL)),
    CHECK ((state = 'ACTIVE' AND revoked_by IS NULL AND revoked_at IS NULL AND revocation_reason IS NULL)
        OR (state = 'REVOKED' AND revoked_by IS NOT NULL AND revoked_at IS NOT NULL AND revocation_reason IS NOT NULL)),
    CHECK (revoked_at IS NULL OR revoked_at >= assigned_at)
);
CREATE UNIQUE INDEX platform_role_assignments_one_active_role
ON platform_administration.role_assignments (user_id, role_key) WHERE state = 'ACTIVE';
CREATE INDEX platform_role_assignments_user_history
ON platform_administration.role_assignments (user_id, assigned_at DESC, id DESC);
CREATE INDEX platform_role_assignments_active_owner
ON platform_administration.role_assignments (user_id) WHERE state = 'ACTIVE' AND role_key = 'PLATFORM_OWNER';

CREATE TABLE platform_administration.administrator_invitations (
    id uuid PRIMARY KEY,
    email text NOT NULL CHECK (length(email) BETWEEN 3 AND 254 AND email = btrim(email)),
    email_lookup text NOT NULL CHECK (
        length(email_lookup) BETWEEN 3 AND 254
        AND email_lookup = lower(email_lookup)
        AND email_lookup !~ '[^\x20-\x7E]'
    ),
    role_key text NOT NULL CHECK (role_key IN (
        'PLATFORM_OWNER','OPERATIONS_ADMINISTRATOR','BILLING_ADMINISTRATOR',
        'INTEGRATION_ADMINISTRATOR','SUPPORT_ADMINISTRATOR','SECURITY_AUDIT_ADMINISTRATOR'
    )),
    authorization_path text NOT NULL CHECK (authorization_path IN ('OWNER_DIRECT','FIRST_OWNER_EXCEPTION','MAKER_CHECKER')),
    state text NOT NULL CHECK (state IN ('PENDING_APPROVAL','ACTIVE','ACCEPTED','CANCELLED','EXPIRED','REPLACED')),
    token_hash bytea UNIQUE CHECK (token_hash IS NULL OR octet_length(token_hash) = 32),
    invited_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    accepted_by uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
    reason text NOT NULL CHECK (length(reason) BETWEEN 1 AND 2000 AND reason = btrim(reason)),
    created_at timestamptz NOT NULL,
    activated_at timestamptz,
    expires_at timestamptz,
    accepted_at timestamptz,
    cancelled_at timestamptz,
    replaced_at timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK ((role_key = 'PLATFORM_OWNER' AND authorization_path IN ('FIRST_OWNER_EXCEPTION','MAKER_CHECKER'))
        OR (role_key <> 'PLATFORM_OWNER' AND authorization_path = 'OWNER_DIRECT')),
    CHECK ((state = 'PENDING_APPROVAL' AND authorization_path = 'MAKER_CHECKER' AND token_hash IS NULL AND activated_at IS NULL AND expires_at IS NULL AND accepted_by IS NULL AND accepted_at IS NULL AND cancelled_at IS NULL AND replaced_at IS NULL)
        OR (state = 'ACTIVE' AND token_hash IS NOT NULL AND activated_at IS NOT NULL AND expires_at = activated_at + interval '72 hours' AND accepted_by IS NULL AND accepted_at IS NULL AND cancelled_at IS NULL AND replaced_at IS NULL)
        OR (state = 'ACCEPTED' AND token_hash IS NOT NULL AND activated_at IS NOT NULL AND expires_at = activated_at + interval '72 hours' AND accepted_by IS NOT NULL AND accepted_at IS NOT NULL AND cancelled_at IS NULL AND replaced_at IS NULL)
        OR (state = 'CANCELLED' AND accepted_by IS NULL AND accepted_at IS NULL AND cancelled_at IS NOT NULL AND replaced_at IS NULL)
        OR (state = 'EXPIRED' AND accepted_by IS NULL AND accepted_at IS NULL AND expires_at IS NOT NULL AND cancelled_at IS NULL AND replaced_at IS NULL)
        OR (state = 'REPLACED' AND accepted_by IS NULL AND accepted_at IS NULL AND cancelled_at IS NULL AND replaced_at IS NOT NULL)),
    CHECK (activated_at IS NULL OR activated_at >= created_at),
    CHECK (accepted_at IS NULL OR accepted_at >= created_at),
    CHECK (cancelled_at IS NULL OR cancelled_at >= created_at),
    CHECK (replaced_at IS NULL OR replaced_at >= created_at),
    CHECK ((accepted_by IS NULL) = (accepted_at IS NULL))
);
CREATE UNIQUE INDEX platform_invitations_one_open_email
ON platform_administration.administrator_invitations (email_lookup)
WHERE state IN ('PENDING_APPROVAL','ACTIVE');
CREATE INDEX platform_invitations_state_time
ON platform_administration.administrator_invitations (state, created_at DESC, id DESC);

CREATE TABLE platform_administration.administrator_changes (
    id uuid PRIMARY KEY,
    action text NOT NULL CHECK (action IN (
        'ASSIGN_ROLE','REVOKE_ROLE','DISABLE_IDENTITY','REACTIVATE_IDENTITY','ACTIVATE_OWNER_INVITATION'
    )),
    target_user_id uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
    invitation_id uuid REFERENCES platform_administration.administrator_invitations(id) ON DELETE RESTRICT,
    role_key text CHECK (role_key IS NULL OR role_key IN (
        'PLATFORM_OWNER','OPERATIONS_ADMINISTRATOR','BILLING_ADMINISTRATOR',
        'INTEGRATION_ADMINISTRATOR','SUPPORT_ADMINISTRATOR','SECURITY_AUDIT_ADMINISTRATOR'
    )),
    assignment_id uuid REFERENCES platform_administration.role_assignments(id) ON DELETE RESTRICT,
    expected_user_version bigint CHECK (expected_user_version IS NULL OR expected_user_version > 0),
    expected_target_context_generation bigint CHECK (expected_target_context_generation IS NULL OR expected_target_context_generation > 0),
    expected_assignment_version bigint CHECK (expected_assignment_version IS NULL OR expected_assignment_version > 0),
    reason text NOT NULL CHECK (length(reason) BETWEEN 1 AND 2000 AND reason = btrim(reason)),
    decision_reason text CHECK (decision_reason IS NULL OR (length(decision_reason) BETWEEN 1 AND 2000 AND decision_reason = btrim(decision_reason))),
    state text NOT NULL CHECK (state IN ('PENDING_APPROVAL','EXECUTED','REJECTED','EXPIRED')),
    maker_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    checker_id uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    expires_at timestamptz,
    decided_at timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK ((state = 'PENDING_APPROVAL' AND decision_reason IS NULL AND checker_id IS NULL AND decided_at IS NULL AND expires_at = created_at + interval '30 minutes')
        OR (state = 'EXECUTED' AND decided_at IS NOT NULL)
        OR (state = 'REJECTED' AND decision_reason IS NOT NULL AND checker_id IS NOT NULL AND decided_at IS NOT NULL)
        OR (state = 'EXPIRED' AND decision_reason IS NULL AND decided_at IS NOT NULL)),
    CHECK (checker_id IS NULL OR checker_id <> maker_id),
    CHECK (decided_at IS NULL OR decided_at >= created_at),
    CHECK ((action = 'ASSIGN_ROLE' AND target_user_id IS NOT NULL AND invitation_id IS NULL AND role_key IS NOT NULL AND assignment_id IS NULL AND expected_user_version IS NOT NULL AND expected_target_context_generation IS NOT NULL AND expected_assignment_version IS NULL)
        OR (action = 'REVOKE_ROLE' AND target_user_id IS NOT NULL AND invitation_id IS NULL AND role_key IS NOT NULL AND assignment_id IS NOT NULL AND expected_user_version IS NOT NULL AND expected_target_context_generation IS NOT NULL AND expected_assignment_version IS NOT NULL)
        OR (action IN ('DISABLE_IDENTITY','REACTIVATE_IDENTITY') AND target_user_id IS NOT NULL AND invitation_id IS NULL AND role_key IS NULL AND assignment_id IS NULL AND expected_user_version IS NOT NULL AND expected_target_context_generation IS NOT NULL AND expected_assignment_version IS NULL)
        OR (action = 'ACTIVATE_OWNER_INVITATION' AND target_user_id IS NULL AND invitation_id IS NOT NULL AND role_key = 'PLATFORM_OWNER' AND assignment_id IS NULL AND expected_user_version IS NULL AND expected_target_context_generation IS NULL AND expected_assignment_version IS NULL))
);
CREATE INDEX platform_changes_state_expiry
ON platform_administration.administrator_changes (state, expires_at, created_at DESC, id DESC);
CREATE INDEX platform_changes_target_history
ON platform_administration.administrator_changes (target_user_id, created_at DESC, id DESC)
WHERE target_user_id IS NOT NULL;
CREATE UNIQUE INDEX platform_changes_one_pending_invitation
ON platform_administration.administrator_changes (invitation_id)
WHERE invitation_id IS NOT NULL AND state = 'PENDING_APPROVAL';

CREATE TABLE platform_administration.commands (
    id uuid PRIMARY KEY,
    actor_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    operation text NOT NULL CHECK (operation IN (
        'INVITATION_ISSUE','INVITATION_CANCEL','INVITATION_ACCEPT',
        'ADMINISTRATOR_CHANGE_CREATE','ADMINISTRATOR_CHANGE_APPROVE','ADMINISTRATOR_CHANGE_REJECT',
        'ADMINISTRATOR_SESSIONS_REVOKE'
    )),
    key_hash bytea NOT NULL CHECK (octet_length(key_hash) = 32),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    target_id uuid,
    response_status integer NOT NULL CHECK (response_status IN (200,201,204)),
    created_at timestamptz NOT NULL,
    completed_at timestamptz NOT NULL,
    UNIQUE (actor_user_id, operation, key_hash),
    CHECK (completed_at >= created_at)
);
CREATE INDEX platform_commands_target
ON platform_administration.commands (target_id, operation) WHERE target_id IS NOT NULL;

CREATE FUNCTION platform_administration.protect_governance_state() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR NEW.singleton IS DISTINCT FROM OLD.singleton
       OR (OLD.bootstrap_consumed AND (NOT NEW.bootstrap_consumed
           OR NEW.bootstrap_owner_id IS DISTINCT FROM OLD.bootstrap_owner_id
           OR NEW.bootstrap_consumed_at IS DISTINCT FROM OLD.bootstrap_consumed_at))
       OR (OLD.first_additional_owner_exception_consumed AND NOT NEW.first_additional_owner_exception_consumed)
       OR NEW.effective_owner_generation < OLD.effective_owner_generation
       OR NEW.version <> OLD.version + 1
       OR NEW.updated_at < OLD.updated_at THEN
        RAISE EXCEPTION 'platform governance transition invalid' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER platform_governance_state_protect
BEFORE UPDATE OR DELETE ON platform_administration.governance_state
FOR EACH ROW EXECUTE FUNCTION platform_administration.protect_governance_state();

CREATE FUNCTION platform_administration.protect_profile() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR NEW.user_id IS DISTINCT FROM OLD.user_id
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR NEW.context_generation < OLD.context_generation
       OR NEW.version <> OLD.version + 1
       OR NEW.updated_at < OLD.updated_at THEN
        RAISE EXCEPTION 'platform administrator profile transition invalid' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER platform_administrator_profiles_protect
BEFORE UPDATE OR DELETE ON platform_administration.administrator_profiles
FOR EACH ROW EXECUTE FUNCTION platform_administration.protect_profile();

CREATE FUNCTION platform_administration.protect_role_assignment() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR NEW.id IS DISTINCT FROM OLD.id OR NEW.user_id IS DISTINCT FROM OLD.user_id
       OR NEW.role_key IS DISTINCT FROM OLD.role_key OR NEW.source IS DISTINCT FROM OLD.source
       OR NEW.assigned_by IS DISTINCT FROM OLD.assigned_by OR NEW.assigned_at IS DISTINCT FROM OLD.assigned_at
       OR NEW.reason IS DISTINCT FROM OLD.reason OR OLD.state <> 'ACTIVE' OR NEW.state <> 'REVOKED'
       OR NEW.revoked_by IS NULL OR NEW.revocation_reason IS NULL OR NEW.revoked_at IS NULL OR NEW.version <> OLD.version + 1 THEN
        RAISE EXCEPTION 'platform role assignment transition invalid' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER platform_role_assignments_protect
BEFORE UPDATE OR DELETE ON platform_administration.role_assignments
FOR EACH ROW EXECUTE FUNCTION platform_administration.protect_role_assignment();

CREATE FUNCTION platform_administration.protect_invitation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR NEW.id IS DISTINCT FROM OLD.id OR NEW.email IS DISTINCT FROM OLD.email
       OR NEW.email_lookup IS DISTINCT FROM OLD.email_lookup OR NEW.role_key IS DISTINCT FROM OLD.role_key
       OR NEW.authorization_path IS DISTINCT FROM OLD.authorization_path OR NEW.invited_by IS DISTINCT FROM OLD.invited_by OR NEW.reason IS DISTINCT FROM OLD.reason
       OR NEW.created_at IS DISTINCT FROM OLD.created_at OR OLD.state IN ('ACCEPTED','CANCELLED','EXPIRED','REPLACED')
       OR ((OLD.state <> 'PENDING_APPROVAL' OR NEW.state <> 'ACTIVE') AND NEW.token_hash IS DISTINCT FROM OLD.token_hash)
       OR (OLD.state = 'ACTIVE' AND (NEW.activated_at IS DISTINCT FROM OLD.activated_at OR NEW.expires_at IS DISTINCT FROM OLD.expires_at))
       OR NEW.version <> OLD.version + 1 THEN
        RAISE EXCEPTION 'platform invitation transition invalid' USING ERRCODE = '55000';
    END IF;
    IF NOT ((OLD.state = 'PENDING_APPROVAL' AND NEW.state IN ('ACTIVE','CANCELLED','EXPIRED','REPLACED'))
        OR (OLD.state = 'ACTIVE' AND NEW.state IN ('ACCEPTED','CANCELLED','EXPIRED','REPLACED'))) THEN
        RAISE EXCEPTION 'platform invitation state transition invalid' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER platform_administrator_invitations_protect
BEFORE UPDATE OR DELETE ON platform_administration.administrator_invitations
FOR EACH ROW EXECUTE FUNCTION platform_administration.protect_invitation();

CREATE FUNCTION platform_administration.protect_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR OLD.state <> 'PENDING_APPROVAL' OR NEW.state NOT IN ('EXECUTED','REJECTED','EXPIRED')
       OR NEW.id IS DISTINCT FROM OLD.id OR NEW.action IS DISTINCT FROM OLD.action
       OR NEW.target_user_id IS DISTINCT FROM OLD.target_user_id OR NEW.invitation_id IS DISTINCT FROM OLD.invitation_id
       OR NEW.role_key IS DISTINCT FROM OLD.role_key OR NEW.assignment_id IS DISTINCT FROM OLD.assignment_id
       OR NEW.expected_user_version IS DISTINCT FROM OLD.expected_user_version
       OR NEW.expected_target_context_generation IS DISTINCT FROM OLD.expected_target_context_generation
       OR NEW.expected_assignment_version IS DISTINCT FROM OLD.expected_assignment_version
       OR NEW.reason IS DISTINCT FROM OLD.reason OR OLD.decision_reason IS NOT NULL
       OR (NEW.state IN ('EXECUTED','REJECTED') AND NEW.decision_reason IS NULL)
       OR NEW.maker_id IS DISTINCT FROM OLD.maker_id OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.expires_at IS DISTINCT FROM OLD.expires_at
       OR (NEW.state <> 'EXPIRED' AND NEW.checker_id IS NULL)
       OR NEW.decided_at IS NULL OR NEW.version <> OLD.version + 1 THEN
        RAISE EXCEPTION 'platform administrator change transition invalid' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER platform_administrator_changes_protect
BEFORE UPDATE OR DELETE ON platform_administration.administrator_changes
FOR EACH ROW EXECUTE FUNCTION platform_administration.protect_change();

CREATE FUNCTION platform_administration.reject_command_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'platform administrator commands are immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER platform_administrator_commands_immutable
BEFORE UPDATE OR DELETE ON platform_administration.commands
FOR EACH ROW EXECUTE FUNCTION platform_administration.reject_command_mutation();

DO $$
DECLARE affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 15, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 14;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 14';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';
UPDATE bidora_control_meta.schema_compatibility
SET schema_version = 14, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 15;
DROP SCHEMA platform_administration CASCADE;
-- +goose StatementEnd
