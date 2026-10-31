-- +goose Up
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE identity.mfa_transactions
    DROP CONSTRAINT mfa_transactions_purpose_check,
    ADD CONSTRAINT mfa_transactions_purpose_check CHECK (purpose IN ('MFA_LOGIN', 'EMAIL_LOGIN')),
    ADD COLUMN email_code_mac bytea,
    ADD CONSTRAINT mfa_transactions_email_code_mac_check CHECK (
        (purpose = 'MFA_LOGIN' AND email_code_mac IS NULL)
        OR (purpose = 'EMAIL_LOGIN' AND octet_length(email_code_mac) = 32)
    );

ALTER TABLE identity.sessions
    DROP CONSTRAINT sessions_assurance_method_check,
    DROP CONSTRAINT session_assurance_shape,
    ADD CONSTRAINT sessions_assurance_method_check CHECK (assurance_method IN ('PRIMARY_ONLY', 'EMAIL_OTP', 'TOTP', 'RECOVERY_CODE')),
    ADD CONSTRAINT session_assurance_shape CHECK (
        (assurance_level = 'AAL1' AND assurance_method IN ('PRIMARY_ONLY', 'EMAIL_OTP'))
        OR (assurance_level = 'AAL2' AND assurance_method IN ('TOTP', 'RECOVERY_CODE'))
    );

COMMENT ON COLUMN identity.mfa_transactions.email_code_mac IS
'Keyed HMAC-SHA-256 of a browser-bound EMAIL_LOGIN code; raw codes are delivery-only and never persisted.';

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=32,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=31;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 31'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';

DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM identity.sessions WHERE assurance_method='EMAIL_OTP') OR EXISTS(SELECT 1 FROM identity.mfa_transactions WHERE purpose='EMAIL_LOGIN') THEN
  RAISE EXCEPTION 'email login history prevents downgrade';
 END IF;
END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=31,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=32;

ALTER TABLE identity.sessions
    DROP CONSTRAINT session_assurance_shape,
    DROP CONSTRAINT sessions_assurance_method_check,
    ADD CONSTRAINT sessions_assurance_method_check CHECK (assurance_method IN ('PRIMARY_ONLY', 'TOTP', 'RECOVERY_CODE')),
    ADD CONSTRAINT session_assurance_shape CHECK (
        (assurance_level = 'AAL1' AND assurance_method = 'PRIMARY_ONLY')
        OR (assurance_level = 'AAL2' AND assurance_method IN ('TOTP', 'RECOVERY_CODE'))
    );

ALTER TABLE identity.mfa_transactions
    DROP CONSTRAINT mfa_transactions_email_code_mac_check,
    DROP COLUMN email_code_mac,
    DROP CONSTRAINT mfa_transactions_purpose_check,
    ADD CONSTRAINT mfa_transactions_purpose_check CHECK (purpose = 'MFA_LOGIN');
-- +goose StatementEnd
