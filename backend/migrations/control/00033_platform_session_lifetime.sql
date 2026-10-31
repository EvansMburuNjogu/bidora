-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';

ALTER TABLE identity.sessions
 DROP CONSTRAINT sessions_check1,
 DROP CONSTRAINT sessions_check2,
 DROP CONSTRAINT sessions_check3,
 ADD COLUMN idle_timeout_seconds integer NOT NULL DEFAULT 1800,
 ADD COLUMN absolute_timeout_seconds integer NOT NULL DEFAULT 43200,
 ADD CONSTRAINT sessions_timeout_profile_check CHECK (
  (idle_timeout_seconds=1800 AND absolute_timeout_seconds=43200)
  OR (idle_timeout_seconds=604800 AND absolute_timeout_seconds=604800)
 ),
 ADD CONSTRAINT sessions_absolute_expires_at_check CHECK (absolute_expires_at=created_at+make_interval(secs=>absolute_timeout_seconds)),
 ADD CONSTRAINT sessions_idle_expires_at_check CHECK (idle_expires_at<=LEAST(last_seen_at+make_interval(secs=>idle_timeout_seconds),absolute_expires_at) AND idle_expires_at>last_seen_at);
ALTER TABLE identity.sessions ALTER COLUMN idle_timeout_seconds DROP DEFAULT, ALTER COLUMN absolute_timeout_seconds DROP DEFAULT;

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=33,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=32;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 32'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM identity.sessions WHERE idle_timeout_seconds<>1800 OR absolute_timeout_seconds<>43200) THEN RAISE EXCEPTION 'extended platform sessions prevent downgrade'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=32,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=33;
ALTER TABLE identity.sessions
 DROP CONSTRAINT sessions_idle_expires_at_check,
 DROP CONSTRAINT sessions_absolute_expires_at_check,
 DROP CONSTRAINT sessions_timeout_profile_check,
 DROP COLUMN absolute_timeout_seconds,
 DROP COLUMN idle_timeout_seconds,
 ADD CONSTRAINT sessions_check1 CHECK (absolute_expires_at=created_at+interval '12 hours'),
 ADD CONSTRAINT sessions_check2 CHECK (idle_expires_at<=LEAST(last_seen_at+interval '30 minutes',absolute_expires_at)),
 ADD CONSTRAINT sessions_check3 CHECK (idle_expires_at>last_seen_at);
-- +goose StatementEnd
