-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';

-- An existing unfenced running job cannot safely be assigned a new owner.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM tenancy.provisioning_jobs WHERE state='RUNNING') THEN
  RAISE EXCEPTION 'existing running provisioning jobs require operator reconciliation';
 END IF;
END $$;

ALTER TABLE tenancy.provisioning_jobs
 ADD COLUMN lease_token uuid,
 ADD COLUMN lease_executor_id uuid,
 ADD CONSTRAINT provisioning_jobs_id_operation_unique UNIQUE (id,operation_id),
 ADD CONSTRAINT provisioning_jobs_id_operation_tenant_unique UNIQUE (id,operation_id,tenant_id),
 ADD CONSTRAINT provisioning_job_lease_fence CHECK (
  (state='RUNNING' AND lease_token IS NOT NULL AND lease_executor_id IS NOT NULL)
  OR (state<>'RUNNING' AND lease_token IS NULL AND lease_executor_id IS NULL)
 );

-- API status needs only these fields. Do not grant API readers the lease-bearing table.
CREATE VIEW tenancy.provisioning_job_status_v1 WITH (security_barrier=true) AS
 SELECT operation_id,state FROM tenancy.provisioning_jobs;
REVOKE ALL ON tenancy.provisioning_job_status_v1 FROM PUBLIC;

CREATE TABLE tenancy.provisioning_job_events (
 id uuid PRIMARY KEY,
 job_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 executor_id uuid NOT NULL,
 lease_token_hash bytea NOT NULL CHECK(octet_length(lease_token_hash)=32),
 FOREIGN KEY(job_id,operation_id) REFERENCES tenancy.provisioning_jobs(id,operation_id) ON DELETE RESTRICT,
 event_type text NOT NULL CHECK(event_type IN ('LEASED','CHECKPOINT_RECORDED','RETRY_SCHEDULED','REVIEW_REQUIRED')),
 checkpoint text NOT NULL CHECK(checkpoint IN ('PLAN_SELECTED','PAYMENT_RECONCILED','DATABASE_RESERVED','DATABASE_CREATED','MIGRATIONS_APPLIED','RUNTIME_GRANTS_APPLIED','COMPATIBILITY_VERIFIED','ASSIGNMENT_ACTIVATED','SUBSCRIPTION_ACTIVATED','ACTIVE')),
 verification_kind text CHECK(verification_kind IS NULL OR verification_kind IN ('PLACEMENT','DATABASE_OWNERSHIP','MIGRATION_LEDGER','RUNTIME_GRANTS','SCHEMA_COMPATIBILITY')),
 verification_digest bytea CHECK(verification_digest IS NULL OR (octet_length(verification_digest)=32 AND verification_digest<>decode(repeat('00',32),'hex'))),
 safe_failure_code text CHECK(safe_failure_code IS NULL OR safe_failure_code IN ('CAPACITY_UNAVAILABLE','DATABASE_RESERVATION_FAILED','DATABASE_CREATION_FAILED','MIGRATION_FAILED','RUNTIME_GRANT_FAILED','COMPATIBILITY_FAILED','ASSIGNMENT_FAILED','SUBSCRIPTION_ACTIVATION_FAILED','EFFECT_AMBIGUOUS','INTERNAL_ERROR')),
 occurred_at timestamptz NOT NULL,
 CHECK ((event_type IN ('RETRY_SCHEDULED','REVIEW_REQUIRED')) = (safe_failure_code IS NOT NULL)),
 CHECK ((event_type='CHECKPOINT_RECORDED' AND verification_kind IS NOT NULL AND verification_digest IS NOT NULL) OR (event_type<>'CHECKPOINT_RECORDED' AND verification_kind IS NULL AND verification_digest IS NULL)),
 CHECK (event_type<>'CHECKPOINT_RECORDED' OR (checkpoint,verification_kind) IN (('DATABASE_RESERVED','PLACEMENT'),('DATABASE_CREATED','DATABASE_OWNERSHIP'),('MIGRATIONS_APPLIED','MIGRATION_LEDGER'),('RUNTIME_GRANTS_APPLIED','RUNTIME_GRANTS'),('COMPATIBILITY_VERIFIED','SCHEMA_COMPATIBILITY')))
);
CREATE INDEX provisioning_job_events_job_idx ON tenancy.provisioning_job_events(job_id,occurred_at,id);
CREATE UNIQUE INDEX provisioning_job_unique_lease_idx ON tenancy.provisioning_job_events(job_id,lease_token_hash) WHERE event_type='LEASED';
CREATE FUNCTION tenancy.reject_provisioning_event_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'provisioning evidence is append-only' USING ERRCODE='55000'; END $$;
REVOKE ALL ON FUNCTION tenancy.reject_provisioning_event_mutation() FROM PUBLIC;
CREATE TRIGGER provisioning_job_events_immutable BEFORE UPDATE OR DELETE ON tenancy.provisioning_job_events FOR EACH ROW EXECUTE FUNCTION tenancy.reject_provisioning_event_mutation();

-- The database enforces only the currently implemented Trial transitions.
-- Activation, paid processing and operator resume need separately reviewed rules.
CREATE FUNCTION tenancy.guard_provisioning_job_transition() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE expected text; commercial text;
BEGIN
 IF (NEW.id,NEW.operation_id,NEW.tenant_id,NEW.created_at) IS DISTINCT FROM (OLD.id,OLD.operation_id,OLD.tenant_id,OLD.created_at) THEN
  RAISE EXCEPTION 'provisioning job identity is immutable' USING ERRCODE='55000';
 END IF;
 IF OLD.state IN ('PENDING','RETRYABLE') AND NEW.state='RUNNING' THEN
  SELECT commercial_path INTO commercial FROM tenancy.organization_creation_operations WHERE id=OLD.operation_id AND tenant_id=OLD.tenant_id;
  IF commercial IS DISTINCT FROM 'TRIAL' OR OLD.checkpoint NOT IN ('PLAN_SELECTED','DATABASE_RESERVED','DATABASE_CREATED','MIGRATIONS_APPLIED','RUNTIME_GRANTS_APPLIED') OR (OLD.next_attempt_at IS NOT NULL AND OLD.next_attempt_at>statement_timestamp()) OR NEW.checkpoint<>OLD.checkpoint OR NEW.attempts<>OLD.attempts+1 OR NEW.attempts>3 OR NEW.lease_until<=clock_timestamp() OR NEW.lease_until>clock_timestamp()+interval '300 seconds' OR NEW.lease_token='00000000-0000-0000-0000-000000000000'::uuid OR NEW.lease_executor_id='00000000-0000-0000-0000-000000000000'::uuid THEN
   RAISE EXCEPTION 'invalid provisioning claim' USING ERRCODE='55000';
  END IF;
 ELSIF OLD.state='RUNNING' AND NEW.state='RUNNING' THEN
  IF OLD.lease_until<=clock_timestamp() OR NEW.lease_token IS DISTINCT FROM OLD.lease_token OR NEW.lease_executor_id IS DISTINCT FROM OLD.lease_executor_id OR NEW.checkpoint<>OLD.checkpoint OR NEW.attempts<>OLD.attempts OR NEW.lease_until<OLD.lease_until OR NEW.lease_until>clock_timestamp()+interval '300 seconds' THEN
   RAISE EXCEPTION 'invalid provisioning renewal' USING ERRCODE='55000';
  END IF;
 ELSIF OLD.state='RUNNING' AND NEW.state='PENDING' THEN
  expected:=CASE OLD.checkpoint WHEN 'PLAN_SELECTED' THEN 'DATABASE_RESERVED' WHEN 'DATABASE_RESERVED' THEN 'DATABASE_CREATED' WHEN 'DATABASE_CREATED' THEN 'MIGRATIONS_APPLIED' WHEN 'MIGRATIONS_APPLIED' THEN 'RUNTIME_GRANTS_APPLIED' WHEN 'RUNTIME_GRANTS_APPLIED' THEN 'COMPATIBILITY_VERIFIED' ELSE NULL END;
  IF OLD.lease_until<=clock_timestamp() OR expected IS NULL OR NEW.checkpoint IS DISTINCT FROM expected OR NEW.attempts<>0 THEN
   RAISE EXCEPTION 'invalid provisioning checkpoint transition' USING ERRCODE='55000';
  END IF;
 ELSIF OLD.state='RUNNING' AND NEW.state='REVIEW_REQUIRED' THEN
  IF NEW.checkpoint<>OLD.checkpoint OR NEW.attempts<>OLD.attempts OR NEW.safe_failure_code IS NULL THEN
   RAISE EXCEPTION 'invalid provisioning review transition' USING ERRCODE='55000';
  END IF;
 ELSE
  RAISE EXCEPTION 'unsupported provisioning job transition' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION tenancy.guard_provisioning_job_transition() FROM PUBLIC;
CREATE TRIGGER provisioning_jobs_transition_guard BEFORE UPDATE ON tenancy.provisioning_jobs FOR EACH ROW EXECUTE FUNCTION tenancy.guard_provisioning_job_transition();

-- A transition and its immutable event must commit together. Renewal has no event.
CREATE FUNCTION tenancy.require_provisioning_transition_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE event_name text; fence_hash bytea; event_checkpoint text; expected_executor uuid;
BEGIN
 IF OLD.state IN ('PENDING','RETRYABLE') AND NEW.state='RUNNING' THEN
  event_name:='LEASED'; fence_hash:=pg_catalog.sha256(pg_catalog.uuid_send(NEW.lease_token)); event_checkpoint:=NEW.checkpoint; expected_executor:=NEW.lease_executor_id;
 ELSIF OLD.state='RUNNING' AND NEW.state='PENDING' THEN
  event_name:='CHECKPOINT_RECORDED'; fence_hash:=pg_catalog.sha256(pg_catalog.uuid_send(OLD.lease_token)); event_checkpoint:=NEW.checkpoint; expected_executor:=OLD.lease_executor_id;
 ELSIF OLD.state='RUNNING' AND NEW.state='REVIEW_REQUIRED' THEN
  event_name:='REVIEW_REQUIRED'; fence_hash:=pg_catalog.sha256(pg_catalog.uuid_send(OLD.lease_token)); event_checkpoint:=NEW.checkpoint;
 ELSE RETURN NULL;
 END IF;
 IF NOT EXISTS (SELECT 1 FROM tenancy.provisioning_job_events e WHERE e.job_id=NEW.id AND e.operation_id=NEW.operation_id AND e.event_type=event_name AND e.lease_token_hash=fence_hash AND e.checkpoint=event_checkpoint AND (expected_executor IS NULL OR e.executor_id=expected_executor) AND (event_name<>'REVIEW_REQUIRED' OR e.safe_failure_code=NEW.safe_failure_code)) THEN
  RAISE EXCEPTION 'provisioning transition evidence required' USING ERRCODE='55000';
 END IF;
 RETURN NULL;
END $$;
REVOKE ALL ON FUNCTION tenancy.require_provisioning_transition_event() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER provisioning_jobs_transition_evidence AFTER UPDATE ON tenancy.provisioning_jobs DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION tenancy.require_provisioning_transition_event();

-- A deployment operator owns this mapping. The authenticated login, not an
-- arbitrary caller-provided executor UUID, selects the worker identity.
CREATE TABLE tenancy.provisioning_worker_identities (
 login_role text PRIMARY KEY CHECK(length(login_role) BETWEEN 1 AND 63 AND login_role ~ '^[a-z_][a-z0-9_]*$'),
 executor_id uuid NOT NULL UNIQUE CHECK(executor_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 enabled boolean NOT NULL,
 registered_at timestamptz NOT NULL
);
REVOKE ALL ON tenancy.provisioning_worker_identities FROM PUBLIC;

-- A verifier has a distinct authenticated login and cannot also be a worker.
-- This mapping confers no grants and does not itself issue attestations.
CREATE TABLE tenancy.provisioning_verifier_identities (
 login_role text PRIMARY KEY CHECK(length(login_role) BETWEEN 1 AND 63 AND login_role ~ '^[a-z_][a-z0-9_]*$'),
 verifier_id uuid NOT NULL UNIQUE CHECK(verifier_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 enabled boolean NOT NULL,
 registered_at timestamptz NOT NULL
);
REVOKE ALL ON tenancy.provisioning_verifier_identities FROM PUBLIC;

-- Capacity limits are deployment-owned. No cluster is enabled or seeded by
-- this migration, and a configured quota is not a physical readiness proof.
CREATE TABLE tenancy.provisioning_cluster_quotas (
 cluster_id uuid PRIMARY KEY REFERENCES bidora_tenancy.database_clusters(id) ON DELETE RESTRICT,
 max_tenant_slots integer NOT NULL CHECK(max_tenant_slots>0),
 enabled boolean NOT NULL,
 version bigint NOT NULL CHECK(version>0),
 updated_at timestamptz NOT NULL
);
REVOKE ALL ON tenancy.provisioning_cluster_quotas FROM PUBLIC;
CREATE FUNCTION tenancy.guard_provisioning_cluster_quota() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended('bidora:provisioning-quota:v1',0));
 IF TG_OP='INSERT' AND NEW.version<>1 THEN RAISE EXCEPTION 'invalid quota version' USING ERRCODE='55000'; END IF;
 IF TG_OP='UPDATE' AND (NEW.cluster_id IS DISTINCT FROM OLD.cluster_id OR NEW.version<>OLD.version+1 OR NEW.updated_at<OLD.updated_at) THEN RAISE EXCEPTION 'invalid quota update' USING ERRCODE='55000'; END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION tenancy.guard_provisioning_cluster_quota() FROM PUBLIC;
CREATE TRIGGER provisioning_cluster_quota_guard BEFORE INSERT OR UPDATE ON tenancy.provisioning_cluster_quotas FOR EACH ROW EXECUTE FUNCTION tenancy.guard_provisioning_cluster_quota();

-- Immutable reservation identity. An allocator/assignment transaction must
-- serialize all capacity consumers and verify availability before insertion.
-- No worker, verifier or browser receives direct mutation privileges here.
CREATE TABLE tenancy.provisioning_placement_reservations (
 id uuid PRIMARY KEY CHECK(id<>'00000000-0000-0000-0000-000000000000'::uuid),
 job_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 tenant_id uuid NOT NULL,
 cluster_id uuid NOT NULL REFERENCES tenancy.provisioning_cluster_quotas(cluster_id) ON DELETE RESTRICT,
 generation bigint NOT NULL CHECK(generation>0),
 cluster_version bigint NOT NULL CHECK(cluster_version>0),
 quota_version bigint NOT NULL CHECK(quota_version>0),
 target_digest bytea NOT NULL CHECK(octet_length(target_digest)=32 AND target_digest<>decode(repeat('00',32),'hex')),
 created_at timestamptz NOT NULL,
 FOREIGN KEY(job_id,operation_id,tenant_id) REFERENCES tenancy.provisioning_jobs(id,operation_id,tenant_id) ON DELETE RESTRICT,
 UNIQUE(job_id),
 UNIQUE(tenant_id,generation),
 UNIQUE(cluster_id,target_digest),
 UNIQUE(id,job_id,operation_id,tenant_id,cluster_id,generation)
);
REVOKE ALL ON tenancy.provisioning_placement_reservations FROM PUBLIC;
CREATE FUNCTION tenancy.guard_provisioning_reservation_insert() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE quota tenancy.provisioning_cluster_quotas%ROWTYPE; cluster_version bigint; occupied bigint; available integer; expiry timestamptz;
BEGIN
 PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended('bidora:provisioning-quota:v1',0));
 SELECT q.* INTO quota FROM tenancy.provisioning_cluster_quotas q WHERE q.cluster_id=NEW.cluster_id AND q.enabled AND q.version=NEW.quota_version FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'placement quota unavailable' USING ERRCODE='55000'; END IF;
 SELECT c.version INTO cluster_version FROM bidora_tenancy.database_clusters c WHERE c.id=NEW.cluster_id AND c.state='ACTIVE' AND c.version=NEW.cluster_version FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'placement cluster unavailable' USING ERRCODE='55000'; END IF;
 SELECT o.available_slots,o.expires_at INTO available,expiry FROM tenancy.provisioning_capacity_observations o WHERE o.cluster_id=NEW.cluster_id AND o.quota_version=NEW.quota_version AND o.cluster_version=NEW.cluster_version AND o.expires_at>clock_timestamp() ORDER BY o.observed_at DESC,o.id DESC LIMIT 1;
 IF NOT FOUND THEN RAISE EXCEPTION 'placement observation unavailable' USING ERRCODE='55000'; END IF;
 SELECT (SELECT count(*) FROM tenancy.provisioning_placement_reservations r WHERE r.cluster_id=NEW.cluster_id)+(SELECT count(*) FROM bidora_tenancy.tenant_database_assignments a WHERE a.cluster_id=NEW.cluster_id AND a.active) INTO occupied;
 IF occupied>=quota.max_tenant_slots OR occupied>=available OR expiry<=clock_timestamp() THEN RAISE EXCEPTION 'placement capacity unavailable' USING ERRCODE='55000'; END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION tenancy.guard_provisioning_reservation_insert() FROM PUBLIC;
CREATE TRIGGER provisioning_reservations_insert_guard BEFORE INSERT ON tenancy.provisioning_placement_reservations FOR EACH ROW EXECUTE FUNCTION tenancy.guard_provisioning_reservation_insert();
CREATE TRIGGER provisioning_reservations_immutable BEFORE UPDATE OR DELETE ON tenancy.provisioning_placement_reservations FOR EACH ROW EXECUTE FUNCTION tenancy.reject_provisioning_event_mutation();

-- Opting a cluster into quota management serializes assignment publication on
-- the same quota row and requires the pre-existing exact reservation. Existing
-- clusters without a quota retain their previous assignment behavior.
CREATE FUNCTION tenancy.guard_quota_managed_assignment() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE found_quota uuid;
BEGIN
 IF NOT NEW.active THEN RETURN NEW; END IF;
 PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended('bidora:provisioning-quota:v1',0));
 SELECT q.cluster_id INTO found_quota FROM tenancy.provisioning_cluster_quotas q WHERE q.cluster_id=NEW.cluster_id FOR UPDATE;
 IF FOUND THEN
  IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles owner_role WHERE owner_role.rolname=current_user AND NOT owner_role.rolcanlogin AND NOT owner_role.rolsuper AND NOT owner_role.rolcreatedb AND NOT owner_role.rolcreaterole AND NOT owner_role.rolreplication AND NOT owner_role.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=owner_role.oid)) THEN RAISE EXCEPTION 'provisioning function owner invalid' USING ERRCODE='42501'; END IF;
  IF NOT EXISTS (SELECT 1 FROM tenancy.provisioning_placement_reservations r WHERE r.cluster_id=NEW.cluster_id AND r.tenant_id=NEW.tenant_id AND r.generation=NEW.generation) THEN RAISE EXCEPTION 'quota-managed assignment requires reservation' USING ERRCODE='55000'; END IF;
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION tenancy.guard_quota_managed_assignment() FROM PUBLIC;
CREATE TRIGGER quota_managed_assignment_guard BEFORE INSERT OR UPDATE OF active,cluster_id,tenant_id,generation ON bidora_tenancy.tenant_database_assignments FOR EACH ROW EXECUTE FUNCTION tenancy.guard_quota_managed_assignment();

-- A verifier assertion is short-lived and independently authenticated. It is
-- not itself proof that an external capacity inspection actually occurred.
CREATE TABLE tenancy.provisioning_capacity_observations (
 id uuid PRIMARY KEY CHECK(id<>'00000000-0000-0000-0000-000000000000'::uuid),
 cluster_id uuid NOT NULL REFERENCES tenancy.provisioning_cluster_quotas(cluster_id) ON DELETE RESTRICT,
 quota_version bigint NOT NULL CHECK(quota_version>0),
 cluster_version bigint NOT NULL CHECK(cluster_version>0),
 verifier_id uuid NOT NULL REFERENCES tenancy.provisioning_verifier_identities(verifier_id) ON DELETE RESTRICT,
 available_slots integer NOT NULL CHECK(available_slots>=0),
 evidence_digest bytea NOT NULL CHECK(octet_length(evidence_digest)=32 AND evidence_digest<>decode(repeat('00',32),'hex')),
 observed_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL CHECK(expires_at>observed_at AND expires_at<=observed_at+interval '2 minutes')
);
CREATE INDEX provisioning_capacity_observations_current_idx ON tenancy.provisioning_capacity_observations(cluster_id,observed_at DESC,id DESC);
REVOKE ALL ON tenancy.provisioning_capacity_observations FROM PUBLIC;
CREATE TRIGGER provisioning_capacity_observations_immutable BEFORE UPDATE OR DELETE ON tenancy.provisioning_capacity_observations FOR EACH ROW EXECUTE FUNCTION tenancy.reject_provisioning_event_mutation();

-- This is an immutable verifier assertion, not proof that any external
-- resource exists. No advance function consumes it until authoritative target
-- reservation and independently verified evidence are implemented.
CREATE TABLE tenancy.provisioning_verifier_attestations (
 id uuid PRIMARY KEY CHECK(id<>'00000000-0000-0000-0000-000000000000'::uuid),
 job_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 tenant_id uuid NOT NULL,
 reservation_id uuid NOT NULL,
 cluster_id uuid NOT NULL,
 reservation_generation bigint NOT NULL CHECK(reservation_generation>0),
 lease_token_hash bytea NOT NULL CHECK(octet_length(lease_token_hash)=32),
 from_checkpoint text NOT NULL,
 next_checkpoint text NOT NULL,
 verification_kind text NOT NULL,
 verification_digest bytea NOT NULL CHECK(octet_length(verification_digest)=32 AND verification_digest<>decode(repeat('00',32),'hex')),
 verifier_id uuid NOT NULL REFERENCES tenancy.provisioning_verifier_identities(verifier_id) ON DELETE RESTRICT,
 issued_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL CHECK(expires_at>issued_at AND expires_at<=issued_at+interval '2 minutes'),
 FOREIGN KEY(job_id,operation_id,tenant_id) REFERENCES tenancy.provisioning_jobs(id,operation_id,tenant_id) ON DELETE RESTRICT,
 FOREIGN KEY(reservation_id,job_id,operation_id,tenant_id,cluster_id,reservation_generation) REFERENCES tenancy.provisioning_placement_reservations(id,job_id,operation_id,tenant_id,cluster_id,generation) ON DELETE RESTRICT,
 UNIQUE(job_id,lease_token_hash,next_checkpoint),
 CHECK ((from_checkpoint,next_checkpoint,verification_kind) IN (
  ('PLAN_SELECTED','DATABASE_RESERVED','PLACEMENT'),
  ('DATABASE_RESERVED','DATABASE_CREATED','DATABASE_OWNERSHIP'),
  ('DATABASE_CREATED','MIGRATIONS_APPLIED','MIGRATION_LEDGER'),
  ('MIGRATIONS_APPLIED','RUNTIME_GRANTS_APPLIED','RUNTIME_GRANTS'),
  ('RUNTIME_GRANTS_APPLIED','COMPATIBILITY_VERIFIED','SCHEMA_COMPATIBILITY')
 ))
);
REVOKE ALL ON tenancy.provisioning_verifier_attestations FROM PUBLIC;
CREATE TRIGGER provisioning_verifier_attestations_immutable BEFORE UPDATE OR DELETE ON tenancy.provisioning_verifier_attestations FOR EACH ROW EXECUTE FUNCTION tenancy.reject_provisioning_event_mutation();

CREATE FUNCTION tenancy.provisioning_worker_executor() RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE executor uuid;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles owner_role WHERE owner_role.rolname=current_user AND NOT owner_role.rolcanlogin AND NOT owner_role.rolsuper AND NOT owner_role.rolcreatedb AND NOT owner_role.rolcreaterole AND NOT owner_role.rolreplication AND NOT owner_role.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=owner_role.oid)) THEN RAISE EXCEPTION 'provisioning function owner invalid' USING ERRCODE='42501'; END IF;
 SELECT w.executor_id INTO executor FROM tenancy.provisioning_worker_identities w
 JOIN pg_catalog.pg_roles r ON r.rolname=w.login_role
 WHERE w.login_role=session_user::text AND w.enabled AND r.rolcanlogin
   AND NOT EXISTS(SELECT 1 FROM tenancy.provisioning_verifier_identities v WHERE v.login_role=w.login_role)
   AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication AND NOT r.rolbypassrls
   AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid)
   AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_class x WHERE x.relowner=r.oid)
   AND NOT pg_catalog.has_database_privilege(r.oid,current_database(),'CREATE')
   AND NOT pg_catalog.has_database_privilege(r.oid,current_database(),'TEMPORARY')
   AND NOT pg_catalog.has_schema_privilege(r.oid,'tenancy','CREATE')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_jobs','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_jobs','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_job_events','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_job_events','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_worker_identities','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_worker_identities','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_verifier_identities','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_verifier_identities','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_verifier_attestations','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_verifier_attestations','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_cluster_quotas','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_cluster_quotas','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_placement_reservations','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_placement_reservations','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_capacity_observations','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_capacity_observations','DELETE,TRUNCATE,REFERENCES,TRIGGER');
 IF executor IS NULL THEN RAISE EXCEPTION 'provisioning worker unavailable' USING ERRCODE='42501'; END IF;
 RETURN executor;
END $$;
REVOKE ALL ON FUNCTION tenancy.provisioning_worker_executor() FROM PUBLIC;

-- For use only from future narrowly scoped SECURITY DEFINER verification
-- functions. No public issuance or checkpoint-advance function exists yet.
CREATE FUNCTION tenancy.provisioning_verifier_identity() RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE verifier uuid;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles owner_role WHERE owner_role.rolname=current_user AND NOT owner_role.rolcanlogin AND NOT owner_role.rolsuper AND NOT owner_role.rolcreatedb AND NOT owner_role.rolcreaterole AND NOT owner_role.rolreplication AND NOT owner_role.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=owner_role.oid)) THEN RAISE EXCEPTION 'provisioning function owner invalid' USING ERRCODE='42501'; END IF;
 SELECT v.verifier_id INTO verifier FROM tenancy.provisioning_verifier_identities v
 JOIN pg_catalog.pg_roles r ON r.rolname=v.login_role
 WHERE v.login_role=session_user::text AND v.enabled AND r.rolcanlogin
   AND NOT EXISTS(SELECT 1 FROM tenancy.provisioning_worker_identities w WHERE w.login_role=v.login_role)
   AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication AND NOT r.rolbypassrls
   AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid)
   AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_class x WHERE x.relowner=r.oid)
   AND NOT pg_catalog.has_database_privilege(r.oid,current_database(),'CREATE')
   AND NOT pg_catalog.has_database_privilege(r.oid,current_database(),'TEMPORARY')
   AND NOT pg_catalog.has_schema_privilege(r.oid,'tenancy','CREATE')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_jobs','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_jobs','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_job_events','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_job_events','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_worker_identities','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_worker_identities','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_verifier_identities','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_verifier_identities','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_verifier_attestations','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_verifier_attestations','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_cluster_quotas','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_cluster_quotas','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_placement_reservations','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_placement_reservations','DELETE,TRUNCATE,REFERENCES,TRIGGER')
   AND NOT pg_catalog.has_any_column_privilege(r.oid,'tenancy.provisioning_capacity_observations','SELECT,INSERT,UPDATE')
   AND NOT pg_catalog.has_table_privilege(r.oid,'tenancy.provisioning_capacity_observations','DELETE,TRUNCATE,REFERENCES,TRIGGER');
 IF verifier IS NULL THEN RAISE EXCEPTION 'provisioning verifier unavailable' USING ERRCODE='42501'; END IF;
 RETURN verifier;
END $$;
REVOKE ALL ON FUNCTION tenancy.provisioning_verifier_identity() FROM PUBLIC;

-- Only an independently authenticated verifier may record an observation.
-- Deployment must provide the authoritative capacity inspector; a digest alone
-- does not establish actual availability and no role is granted here.
CREATE FUNCTION tenancy.record_provisioning_capacity(p_id uuid,p_cluster uuid,p_quota_version bigint,p_cluster_version bigint,p_available integer,p_digest bytea)
RETURNS TABLE(observation_id uuid,expires_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE verifier uuid; quota_limit integer; observed timestamptz;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles owner_role WHERE owner_role.rolname=current_user AND NOT owner_role.rolcanlogin AND NOT owner_role.rolsuper AND NOT owner_role.rolcreatedb AND NOT owner_role.rolcreaterole AND NOT owner_role.rolreplication AND NOT owner_role.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=owner_role.oid)) THEN RAISE EXCEPTION 'provisioning function owner invalid' USING ERRCODE='42501'; END IF;
 verifier:=tenancy.provisioning_verifier_identity();
 IF p_id IS NULL OR p_id='00000000-0000-0000-0000-000000000000'::uuid OR p_cluster IS NULL OR p_quota_version IS NULL OR p_quota_version<1 OR p_cluster_version IS NULL OR p_cluster_version<1 OR p_available IS NULL OR p_available<0 OR p_digest IS NULL OR octet_length(p_digest)<>32 OR p_digest=decode(repeat('00',32),'hex') THEN RAISE EXCEPTION 'invalid capacity observation' USING ERRCODE='22023'; END IF;
 SELECT q.max_tenant_slots INTO quota_limit FROM tenancy.provisioning_cluster_quotas q JOIN bidora_tenancy.database_clusters c ON c.id=q.cluster_id
 WHERE q.cluster_id=p_cluster AND q.enabled AND q.version=p_quota_version AND c.state='ACTIVE' AND c.version=p_cluster_version FOR SHARE OF q,c;
 IF NOT FOUND OR p_available>quota_limit THEN RAISE EXCEPTION 'capacity policy unavailable' USING ERRCODE='55000'; END IF;
 observed:=clock_timestamp();
 INSERT INTO tenancy.provisioning_capacity_observations(id,cluster_id,quota_version,cluster_version,verifier_id,available_slots,evidence_digest,observed_at,expires_at)
 VALUES(p_id,p_cluster,p_quota_version,p_cluster_version,verifier,p_available,p_digest,observed,observed+interval '2 minutes') ON CONFLICT(id) DO NOTHING;
 RETURN QUERY SELECT o.id,o.expires_at FROM tenancy.provisioning_capacity_observations o WHERE o.id=p_id AND o.cluster_id=p_cluster AND o.quota_version=p_quota_version AND o.cluster_version=p_cluster_version AND o.verifier_id=verifier AND o.available_slots=p_available AND o.evidence_digest=p_digest AND o.expires_at>clock_timestamp();
 IF NOT FOUND THEN RAISE EXCEPTION 'capacity observation unavailable' USING ERRCODE='55000'; END IF;
END $$;
REVOKE ALL ON FUNCTION tenancy.record_provisioning_capacity(uuid,uuid,bigint,bigint,integer,bytea) FROM PUBLIC;

-- Quota allocation is serialized on the operator-owned cluster quota row.
-- The worker cannot choose the cluster, database name or target identifier.
-- A recorded observation is a verifier assertion, not proof of physical capacity.
CREATE FUNCTION tenancy.reserve_provisioning_placement(p_id uuid,p_job uuid,p_operation uuid,p_tenant uuid,p_token uuid)
RETURNS TABLE(reservation_id uuid,cluster_id uuid,generation bigint,target_digest bytea)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE executor uuid; job tenancy.provisioning_jobs%ROWTYPE; tenant_generation bigint; prior tenancy.provisioning_placement_reservations%ROWTYPE; candidate record; observation record; occupied bigint; digest bytea; observed timestamptz;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles owner_role WHERE owner_role.rolname=current_user AND NOT owner_role.rolcanlogin AND NOT owner_role.rolsuper AND NOT owner_role.rolcreatedb AND NOT owner_role.rolcreaterole AND NOT owner_role.rolreplication AND NOT owner_role.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=owner_role.oid)) THEN RAISE EXCEPTION 'provisioning function owner invalid' USING ERRCODE='42501'; END IF;
 executor:=tenancy.provisioning_worker_executor();
 IF p_id IS NULL OR p_id='00000000-0000-0000-0000-000000000000'::uuid OR p_job IS NULL OR p_operation IS NULL OR p_tenant IS NULL OR p_token IS NULL THEN RAISE EXCEPTION 'invalid placement request' USING ERRCODE='22023'; END IF;
 SELECT j.* INTO job FROM tenancy.provisioning_jobs j WHERE j.id=p_job AND j.operation_id=p_operation AND j.tenant_id=p_tenant AND j.state='RUNNING' AND j.lease_token=p_token AND j.lease_executor_id=executor AND j.checkpoint='PLAN_SELECTED' FOR UPDATE;
 IF NOT FOUND OR job.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'provisioning lease unavailable' USING ERRCODE='55000'; END IF;
 SELECT t.assignment_generation INTO tenant_generation FROM bidora_tenancy.tenants t WHERE t.id=p_tenant AND t.tenant_type='TENDERPRENEUR' AND t.state='REQUESTED' FOR UPDATE;
 IF NOT FOUND OR tenant_generation<>0 THEN RAISE EXCEPTION 'tenant placement unavailable' USING ERRCODE='55000'; END IF;
 PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended('bidora:provisioning-quota:v1',0));
 IF job.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'provisioning lease unavailable' USING ERRCODE='55000'; END IF;
 SELECT r.* INTO prior FROM tenancy.provisioning_placement_reservations r WHERE r.job_id=p_job;
 IF FOUND THEN
  IF prior.id<>p_id OR prior.operation_id<>p_operation OR prior.tenant_id<>p_tenant OR prior.generation<>tenant_generation+1 OR NOT EXISTS(SELECT 1 FROM tenancy.provisioning_cluster_quotas q JOIN bidora_tenancy.database_clusters c ON c.id=q.cluster_id WHERE q.cluster_id=prior.cluster_id AND q.enabled AND q.version=prior.quota_version AND c.state='ACTIVE' AND c.version=prior.cluster_version) OR job.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'placement unavailable' USING ERRCODE='55000'; END IF;
  RETURN QUERY SELECT prior.id,prior.cluster_id,prior.generation,prior.target_digest; RETURN;
 END IF;
 FOR candidate IN SELECT q.cluster_id,q.max_tenant_slots,q.version AS quota_version,c.version AS cluster_version FROM tenancy.provisioning_cluster_quotas q JOIN bidora_tenancy.database_clusters c ON c.id=q.cluster_id WHERE q.enabled AND c.state='ACTIVE' ORDER BY q.cluster_id FOR UPDATE OF q SKIP LOCKED LOOP
  PERFORM 1 FROM bidora_tenancy.database_clusters c WHERE c.id=candidate.cluster_id AND c.state='ACTIVE' AND c.version=candidate.cluster_version FOR SHARE;
  IF NOT FOUND THEN CONTINUE; END IF;
  SELECT o.available_slots,o.expires_at INTO observation FROM tenancy.provisioning_capacity_observations o WHERE o.cluster_id=candidate.cluster_id AND o.quota_version=candidate.quota_version AND o.cluster_version=candidate.cluster_version AND o.expires_at>clock_timestamp() ORDER BY o.observed_at DESC,o.id DESC LIMIT 1;
  IF NOT FOUND THEN CONTINUE; END IF;
  -- Deliberately conservative: a reservation and later assignment may both
  -- count. We never infer spare capacity from a potentially duplicated count.
  SELECT (SELECT count(*) FROM tenancy.provisioning_placement_reservations r WHERE r.cluster_id=candidate.cluster_id)+(SELECT count(*) FROM bidora_tenancy.tenant_database_assignments a WHERE a.cluster_id=candidate.cluster_id AND a.active) INTO occupied;
  IF occupied>=candidate.max_tenant_slots OR occupied>=observation.available_slots THEN CONTINUE; END IF;
  observed:=clock_timestamp();
  IF job.lease_until<=observed OR observation.expires_at<=observed THEN RAISE EXCEPTION 'placement readiness unavailable' USING ERRCODE='55000'; END IF;
  digest:=pg_catalog.sha256(pg_catalog.convert_to('bidora:placement:v1:','UTF8')||pg_catalog.uuid_send(p_id)||pg_catalog.uuid_send(p_job)||pg_catalog.uuid_send(p_operation)||pg_catalog.uuid_send(p_tenant)||pg_catalog.uuid_send(candidate.cluster_id)||pg_catalog.int8send(tenant_generation+1));
  INSERT INTO tenancy.provisioning_placement_reservations(id,job_id,operation_id,tenant_id,cluster_id,generation,cluster_version,quota_version,target_digest,created_at)
   VALUES(p_id,p_job,p_operation,p_tenant,candidate.cluster_id,tenant_generation+1,candidate.cluster_version,candidate.quota_version,digest,observed);
  RETURN QUERY SELECT p_id,candidate.cluster_id,tenant_generation+1,digest; RETURN;
 END LOOP;
 RAISE EXCEPTION 'placement capacity unavailable' USING ERRCODE='55000';
END $$;
REVOKE ALL ON FUNCTION tenancy.reserve_provisioning_placement(uuid,uuid,uuid,uuid,uuid) FROM PUBLIC;

CREATE FUNCTION tenancy.issue_provisioning_attestation(p_id uuid,p_job uuid,p_operation uuid,p_tenant uuid,p_reservation uuid,p_cluster uuid,p_generation bigint,p_lease_hash bytea,p_next text,p_kind text,p_digest bytea)
RETURNS TABLE(attestation_id uuid,expires_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE verifier uuid; job tenancy.provisioning_jobs%ROWTYPE; expected text; expected_kind text; observed timestamptz; expiry timestamptz;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles owner_role WHERE owner_role.rolname=current_user AND NOT owner_role.rolcanlogin AND NOT owner_role.rolsuper AND NOT owner_role.rolcreatedb AND NOT owner_role.rolcreaterole AND NOT owner_role.rolreplication AND NOT owner_role.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=owner_role.oid)) THEN RAISE EXCEPTION 'provisioning function owner invalid' USING ERRCODE='42501'; END IF;
 verifier:=tenancy.provisioning_verifier_identity();
 IF p_id IS NULL OR p_id='00000000-0000-0000-0000-000000000000'::uuid OR p_job IS NULL OR p_operation IS NULL OR p_tenant IS NULL OR p_reservation IS NULL OR p_cluster IS NULL OR p_generation IS NULL OR p_generation<1 OR p_lease_hash IS NULL OR octet_length(p_lease_hash)<>32 OR p_digest IS NULL OR octet_length(p_digest)<>32 OR p_digest=decode(repeat('00',32),'hex') THEN RAISE EXCEPTION 'invalid attestation request' USING ERRCODE='22023'; END IF;
 SELECT j.* INTO job FROM tenancy.provisioning_jobs j
 WHERE j.id=p_job AND j.operation_id=p_operation AND j.tenant_id=p_tenant AND j.state='RUNNING'
  AND pg_catalog.sha256(pg_catalog.uuid_send(j.lease_token))=p_lease_hash FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'provisioning lease unavailable' USING ERRCODE='55000'; END IF;
 observed:=clock_timestamp();
 IF job.lease_until<=observed THEN RAISE EXCEPTION 'provisioning lease unavailable' USING ERRCODE='55000'; END IF;
 PERFORM 1 FROM tenancy.provisioning_placement_reservations r
 JOIN tenancy.provisioning_cluster_quotas q ON q.cluster_id=r.cluster_id AND q.enabled AND q.version=r.quota_version
 JOIN bidora_tenancy.database_clusters c ON c.id=r.cluster_id AND c.state='ACTIVE' AND c.version=r.cluster_version
 JOIN bidora_tenancy.tenants t ON t.id=r.tenant_id AND t.state IN ('REQUESTED','PROVISIONING','MIGRATING','SEEDING') AND t.assignment_generation=r.generation-1
 WHERE r.id=p_reservation AND r.job_id=p_job AND r.operation_id=p_operation AND r.tenant_id=p_tenant AND r.cluster_id=p_cluster AND r.generation=p_generation
 FOR SHARE OF q,c,t;
 IF NOT FOUND OR job.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'provisioning reservation unavailable' USING ERRCODE='55000'; END IF;
 observed:=clock_timestamp();
 expected:=CASE job.checkpoint WHEN 'PLAN_SELECTED' THEN 'DATABASE_RESERVED' WHEN 'DATABASE_RESERVED' THEN 'DATABASE_CREATED' WHEN 'DATABASE_CREATED' THEN 'MIGRATIONS_APPLIED' WHEN 'MIGRATIONS_APPLIED' THEN 'RUNTIME_GRANTS_APPLIED' WHEN 'RUNTIME_GRANTS_APPLIED' THEN 'COMPATIBILITY_VERIFIED' ELSE NULL END;
 expected_kind:=CASE expected WHEN 'DATABASE_RESERVED' THEN 'PLACEMENT' WHEN 'DATABASE_CREATED' THEN 'DATABASE_OWNERSHIP' WHEN 'MIGRATIONS_APPLIED' THEN 'MIGRATION_LEDGER' WHEN 'RUNTIME_GRANTS_APPLIED' THEN 'RUNTIME_GRANTS' WHEN 'COMPATIBILITY_VERIFIED' THEN 'SCHEMA_COMPATIBILITY' ELSE NULL END;
 IF expected IS NULL OR p_next IS DISTINCT FROM expected OR p_kind IS DISTINCT FROM expected_kind THEN RAISE EXCEPTION 'invalid attestation checkpoint' USING ERRCODE='22023'; END IF;
 expiry:=LEAST(job.lease_until,observed+interval '2 minutes');
 INSERT INTO tenancy.provisioning_verifier_attestations(id,job_id,operation_id,tenant_id,reservation_id,cluster_id,reservation_generation,lease_token_hash,from_checkpoint,next_checkpoint,verification_kind,verification_digest,verifier_id,issued_at,expires_at)
 VALUES(p_id,p_job,p_operation,p_tenant,p_reservation,p_cluster,p_generation,p_lease_hash,job.checkpoint,p_next,p_kind,p_digest,verifier,observed,expiry)
 ON CONFLICT(id) DO NOTHING;
 RETURN QUERY SELECT a.id,a.expires_at FROM tenancy.provisioning_verifier_attestations a
  WHERE a.id=p_id AND a.job_id=p_job AND a.operation_id=p_operation AND a.tenant_id=p_tenant AND a.reservation_id=p_reservation AND a.cluster_id=p_cluster AND a.reservation_generation=p_generation AND a.lease_token_hash=p_lease_hash
   AND a.from_checkpoint=job.checkpoint AND a.next_checkpoint=p_next AND a.verification_kind=p_kind AND a.verification_digest=p_digest AND a.verifier_id=verifier AND a.expires_at>clock_timestamp();
 IF NOT FOUND THEN RAISE EXCEPTION 'attestation unavailable' USING ERRCODE='55000'; END IF;
END $$;
REVOKE ALL ON FUNCTION tenancy.issue_provisioning_attestation(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bytea,text,text,bytea) FROM PUBLIC;

CREATE FUNCTION tenancy.claim_provisioning_job(p_token uuid,p_event uuid,p_seconds integer)
RETURNS TABLE(id uuid,operation_id uuid,tenant_id uuid,checkpoint text,attempts integer,lease_until timestamptz,executor_id uuid)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE executor uuid;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles owner_role WHERE owner_role.rolname=current_user AND NOT owner_role.rolcanlogin AND NOT owner_role.rolsuper AND NOT owner_role.rolcreatedb AND NOT owner_role.rolcreaterole AND NOT owner_role.rolreplication AND NOT owner_role.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=owner_role.oid)) THEN RAISE EXCEPTION 'provisioning function owner invalid' USING ERRCODE='42501'; END IF;
 executor:=tenancy.provisioning_worker_executor();
 IF p_token IS NULL OR p_token='00000000-0000-0000-0000-000000000000'::uuid OR p_event IS NULL OR p_event='00000000-0000-0000-0000-000000000000'::uuid OR p_seconds IS NULL OR p_seconds NOT BETWEEN 30 AND 300 THEN RAISE EXCEPTION 'invalid provisioning claim' USING ERRCODE='22023'; END IF;
 RETURN QUERY WITH candidate AS (
  SELECT j.id FROM tenancy.provisioning_jobs j JOIN tenancy.organization_creation_operations o ON o.id=j.operation_id
  WHERE j.state IN ('PENDING','RETRYABLE') AND j.attempts<3 AND j.checkpoint IN ('PLAN_SELECTED','DATABASE_RESERVED','DATABASE_CREATED','MIGRATIONS_APPLIED','RUNTIME_GRANTS_APPLIED')
   AND (j.next_attempt_at IS NULL OR j.next_attempt_at<=statement_timestamp()) AND o.commercial_path='TRIAL' AND o.state IN ('REQUESTED','PLAN_SELECTED','PROVISIONING')
  ORDER BY j.next_attempt_at NULLS FIRST,j.id FOR UPDATE OF j SKIP LOCKED LIMIT 1
 ), leased AS (
  UPDATE tenancy.provisioning_jobs j SET state='RUNNING',lease_token=p_token,lease_executor_id=executor,lease_until=clock_timestamp()+make_interval(secs=>p_seconds),attempts=j.attempts+1,next_attempt_at=NULL,safe_failure_code=NULL,updated_at=statement_timestamp()
  FROM candidate c WHERE j.id=c.id RETURNING j.id,j.operation_id,j.tenant_id,j.checkpoint,j.attempts,j.lease_until
 ), evidence AS (
  INSERT INTO tenancy.provisioning_job_events(id,job_id,operation_id,executor_id,lease_token_hash,event_type,checkpoint,occurred_at)
  SELECT p_event,l.id,l.operation_id,executor,pg_catalog.sha256(pg_catalog.uuid_send(p_token)),'LEASED',l.checkpoint,statement_timestamp() FROM leased l RETURNING job_id
 ) SELECT l.id,l.operation_id,l.tenant_id,l.checkpoint,l.attempts,l.lease_until,executor FROM leased l JOIN evidence e ON e.job_id=l.id;
END $$;
REVOKE ALL ON FUNCTION tenancy.claim_provisioning_job(uuid,uuid,integer) FROM PUBLIC;

CREATE FUNCTION tenancy.renew_provisioning_lease(p_job uuid,p_operation uuid,p_tenant uuid,p_token uuid,p_seconds integer) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE executor uuid; changed integer;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles owner_role WHERE owner_role.rolname=current_user AND NOT owner_role.rolcanlogin AND NOT owner_role.rolsuper AND NOT owner_role.rolcreatedb AND NOT owner_role.rolcreaterole AND NOT owner_role.rolreplication AND NOT owner_role.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=owner_role.oid)) THEN RAISE EXCEPTION 'provisioning function owner invalid' USING ERRCODE='42501'; END IF;
 executor:=tenancy.provisioning_worker_executor();
 IF p_job IS NULL OR p_operation IS NULL OR p_tenant IS NULL OR p_token IS NULL OR p_seconds IS NULL OR p_seconds NOT BETWEEN 30 AND 300 THEN RAISE EXCEPTION 'invalid provisioning renewal' USING ERRCODE='22023'; END IF;
 UPDATE tenancy.provisioning_jobs j SET lease_until=GREATEST(j.lease_until,clock_timestamp()+make_interval(secs=>p_seconds)),updated_at=statement_timestamp()
 WHERE j.id=p_job AND j.operation_id=p_operation AND j.tenant_id=p_tenant AND j.state='RUNNING' AND j.lease_token=p_token AND j.lease_executor_id=executor AND j.lease_until>clock_timestamp();
 GET DIAGNOSTICS changed=ROW_COUNT; RETURN changed=1;
END $$;
REVOKE ALL ON FUNCTION tenancy.renew_provisioning_lease(uuid,uuid,uuid,uuid,integer) FROM PUBLIC;

CREATE FUNCTION tenancy.hold_provisioning_job(p_job uuid,p_operation uuid,p_tenant uuid,p_token uuid,p_event uuid,p_reason text) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE executor uuid; changed integer;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles owner_role WHERE owner_role.rolname=current_user AND NOT owner_role.rolcanlogin AND NOT owner_role.rolsuper AND NOT owner_role.rolcreatedb AND NOT owner_role.rolcreaterole AND NOT owner_role.rolreplication AND NOT owner_role.rolbypassrls AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=owner_role.oid)) THEN RAISE EXCEPTION 'provisioning function owner invalid' USING ERRCODE='42501'; END IF;
 executor:=tenancy.provisioning_worker_executor();
 IF p_job IS NULL OR p_operation IS NULL OR p_tenant IS NULL OR p_token IS NULL OR p_event IS NULL OR p_event='00000000-0000-0000-0000-000000000000'::uuid OR p_reason IS NULL OR p_reason NOT IN ('CAPACITY_UNAVAILABLE','DATABASE_RESERVATION_FAILED','DATABASE_CREATION_FAILED','MIGRATION_FAILED','RUNTIME_GRANT_FAILED','COMPATIBILITY_FAILED','ASSIGNMENT_FAILED','SUBSCRIPTION_ACTIVATION_FAILED','EFFECT_AMBIGUOUS','INTERNAL_ERROR') THEN RAISE EXCEPTION 'invalid provisioning hold' USING ERRCODE='22023'; END IF;
 WITH held AS (
  UPDATE tenancy.provisioning_jobs j SET state='REVIEW_REQUIRED',safe_failure_code=p_reason,lease_token=NULL,lease_executor_id=NULL,lease_until=NULL,next_attempt_at=NULL,updated_at=statement_timestamp()
  WHERE j.id=p_job AND j.operation_id=p_operation AND j.tenant_id=p_tenant AND j.state='RUNNING' AND j.lease_token=p_token AND j.lease_executor_id=executor AND j.lease_until>clock_timestamp()
  RETURNING j.id,j.operation_id,j.checkpoint
 ) INSERT INTO tenancy.provisioning_job_events(id,job_id,operation_id,executor_id,lease_token_hash,event_type,checkpoint,safe_failure_code,occurred_at)
 SELECT p_event,h.id,h.operation_id,executor,pg_catalog.sha256(pg_catalog.uuid_send(p_token)),'REVIEW_REQUIRED',h.checkpoint,p_reason,statement_timestamp() FROM held h;
 GET DIAGNOSTICS changed=ROW_COUNT; RETURN changed=1;
END $$;
REVOKE ALL ON FUNCTION tenancy.hold_provisioning_job(uuid,uuid,uuid,uuid,uuid,text) FROM PUBLIC;

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=62,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=61;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 61'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'provisioning evidence is forward-only'; END $$;
-- +goose StatementEnd
