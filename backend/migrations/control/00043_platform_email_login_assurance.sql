-- +goose Up
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';
-- The identity use case, not a caller or a frontend, decides whether a verified
-- email challenge is eligible for administrator assurance. Existing ordinary
-- user sessions remain AAL1 and existing TOTP/recovery semantics are unchanged.
ALTER TABLE identity.sessions DROP CONSTRAINT session_assurance_shape;
ALTER TABLE identity.sessions ADD CONSTRAINT session_assurance_shape CHECK (
 (assurance_level='AAL1' AND assurance_method IN ('PRIMARY_ONLY','EMAIL_OTP')) OR
 (assurance_level='AAL2' AND assurance_method IN ('TOTP','RECOVERY_CODE','EMAIL_OTP'))
);
DO $$ BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=43,updated_at=CURRENT_TIMESTAMP
 WHERE migration_set='control_registry' AND schema_version=42;
 IF NOT FOUND THEN RAISE EXCEPTION 'control compatibility must be at 42'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 RAISE EXCEPTION 'platform email assurance is forward-only; revoke sessions and use a reviewed forward correction';
END $$;
-- +goose StatementEnd
