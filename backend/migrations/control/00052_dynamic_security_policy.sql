-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '30s';
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM bidora_control_meta.schema_compatibility WHERE migration_set='control_registry' AND schema_version=51) THEN RAISE EXCEPTION 'control schema 51 required'; END IF; END $$;
ALTER TABLE platform_settings.session_policy
 ADD COLUMN failed_login_attempts integer NOT NULL DEFAULT 5 CHECK(failed_login_attempts BETWEEN 1 AND 5),
 ADD COLUMN failure_window_seconds integer NOT NULL DEFAULT 900 CHECK(failure_window_seconds BETWEEN 60 AND 3600),
 ADD COLUMN lockout_seconds integer NOT NULL DEFAULT 900 CHECK(lockout_seconds BETWEEN 60 AND 3600),
 ADD COLUMN minimum_password_bytes integer NOT NULL DEFAULT 12 CHECK(minimum_password_bytes BETWEEN 12 AND 128),
 ADD COLUMN password_reset_seconds integer NOT NULL DEFAULT 1800 CHECK(password_reset_seconds BETWEEN 300 AND 1800),
 ADD COLUMN allow_multiple_sessions boolean NOT NULL DEFAULT true,
 ADD COLUMN admin_idle_seconds integer NOT NULL DEFAULT 604800 CHECK(admin_idle_seconds BETWEEN 300 AND 604800),
 ADD COLUMN admin_absolute_seconds integer NOT NULL DEFAULT 604800 CHECK(admin_absolute_seconds BETWEEN 3600 AND 604800),
 ADD CONSTRAINT administrator_session_policy_order CHECK(admin_absolute_seconds>=admin_idle_seconds),
 ADD CONSTRAINT security_policy_minute_precision CHECK(failure_window_seconds%60=0 AND lockout_seconds%60=0 AND password_reset_seconds%60=0 AND admin_idle_seconds%60=0 AND admin_absolute_seconds%60=0);
-- Serialize sign-ins per account; this applies to every password/Google/MFA
-- session writer, including rotations, without giving runtime raw policy access.
-- +goose StatementBegin
CREATE FUNCTION identity.enforce_single_session_policy() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE p platform_settings.session_policy%ROWTYPE; classification text; epoch bigint; verified timestamptz; disabled timestamptz; idle_seconds integer; absolute_seconds integer;
BEGIN
 SELECT account_type,auth_epoch,email_verified_at,disabled_at INTO classification,epoch,verified,disabled FROM identity.users WHERE id=NEW.user_id FOR UPDATE;
 IF NOT FOUND OR disabled IS NOT NULL OR verified IS NULL OR epoch<>NEW.auth_epoch THEN RAISE EXCEPTION 'session identity changed' USING ERRCODE='55000'; END IF;
 SELECT * INTO p FROM platform_settings.session_policy WHERE id=true FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'security policy missing' USING ERRCODE='55000'; END IF;
 IF classification='PLATFORM_ADMIN' AND NEW.assurance_level='AAL2' AND NEW.assurance_method IN('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN idle_seconds:=p.admin_idle_seconds;absolute_seconds:=p.admin_absolute_seconds;
 ELSE idle_seconds:=p.idle_seconds;absolute_seconds:=p.absolute_seconds;END IF;
 NEW.idle_timeout_seconds:=LEAST(NEW.idle_timeout_seconds,idle_seconds);
 NEW.absolute_timeout_seconds:=LEAST(NEW.absolute_timeout_seconds,absolute_seconds);
 NEW.absolute_expires_at:=NEW.created_at+make_interval(secs=>NEW.absolute_timeout_seconds);
 NEW.idle_expires_at:=LEAST(NEW.idle_expires_at,NEW.last_seen_at+make_interval(secs=>NEW.idle_timeout_seconds),NEW.absolute_expires_at);
 IF NOT p.allow_multiple_sessions THEN
  UPDATE identity.sessions SET revoked_at=GREATEST(NEW.created_at,created_at,last_seen_at) WHERE user_id=NEW.user_id AND id<>NEW.id AND revoked_at IS NULL;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION identity.enforce_single_session_policy() FROM PUBLIC;
CREATE TRIGGER enforce_single_session_policy BEFORE INSERT ON identity.sessions FOR EACH ROW EXECUTE FUNCTION identity.enforce_single_session_policy();
-- Password recovery may be tightened, never extended beyond the existing cap.
ALTER TABLE identity.password_recovery_challenges DROP CONSTRAINT password_recovery_challenges_check;
ALTER TABLE identity.sessions DROP CONSTRAINT sessions_timeout_profile_check;
ALTER TABLE identity.sessions ADD CONSTRAINT sessions_timeout_profile_check CHECK(idle_timeout_seconds BETWEEN 300 AND 604800 AND absolute_timeout_seconds BETWEEN 3600 AND 604800 AND absolute_timeout_seconds>=idle_timeout_seconds);
ALTER TABLE identity.password_recovery_challenges ADD CONSTRAINT password_recovery_expiry_bounds CHECK(expires_at>=created_at+interval '5 minutes' AND expires_at<=created_at+interval '30 minutes');
UPDATE bidora_control_meta.schema_compatibility SET schema_version=52,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=51;
-- +goose Down
-- Forward-only: preserve settings and recovery/session evidence.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'dynamic security policy requires forward recovery'; END $$;
-- +goose StatementEnd
