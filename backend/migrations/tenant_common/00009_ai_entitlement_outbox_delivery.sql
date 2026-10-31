-- +goose Up
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE bidora_eventing.outbox_events
  ADD COLUMN delivery_lease_id uuid,
  ADD COLUMN delivery_lease_expires_at timestamptz,
  ADD COLUMN delivery_hold_code text,
  ADD CONSTRAINT outbox_delivery_hold_code CHECK (delivery_hold_code IS NULL OR delivery_hold_code='JETSTREAM_DELIVERY_EXHAUSTED'),
  ADD CONSTRAINT outbox_delivery_lease_pair CHECK ((delivery_lease_id IS NULL) = (delivery_lease_expires_at IS NULL));

CREATE INDEX outbox_ai_entitlement_delivery_idx
ON bidora_eventing.outbox_events(next_attempt_at, id)
WHERE published_at IS NULL AND delivery_hold_code IS NULL AND event_type='tenant.ai_entitlement.reserve_requested.v1';

DO $$
DECLARE affected integer;
BEGIN
  UPDATE bidora_meta.schema_compatibility SET schema_version=9,updated_at=CURRENT_TIMESTAMP
  WHERE migration_set='tenant_common' AND schema_version=8;
  GET DIAGNOSTICS affected = ROW_COUNT;
  IF affected <> 1 THEN RAISE EXCEPTION 'tenant_common compatibility row missing or not at version 8'; END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=8,updated_at=CURRENT_TIMESTAMP
WHERE migration_set='tenant_common' AND schema_version=9;
DROP INDEX bidora_eventing.outbox_ai_entitlement_delivery_idx;
ALTER TABLE bidora_eventing.outbox_events DROP CONSTRAINT outbox_delivery_lease_pair,
  DROP CONSTRAINT outbox_delivery_hold_code,
  DROP COLUMN delivery_hold_code,DROP COLUMN delivery_lease_expires_at,DROP COLUMN delivery_lease_id;
-- +goose StatementEnd
