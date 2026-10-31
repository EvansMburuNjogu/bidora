package persistence

import (
	"context"
	"errors"
	"time"

	controlsql "bidora/backend/internal/controlplane/persistence/sqlc"
	dispatchapp "bidora/backend/internal/entitlementdispatch/application"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type dispatchTransactionRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}

type AIEntitlementAcknowledgementOutbox struct{ runner dispatchTransactionRunner }

func NewAIEntitlementAcknowledgementOutbox(runner dispatchTransactionRunner) (*AIEntitlementAcknowledgementOutbox, error) {
	if runner == nil {
		return nil, errors.New("ai_entitlement_acknowledgement_outbox_invalid")
	}
	return &AIEntitlementAcknowledgementOutbox{runner: runner}, nil
}

func (s *AIEntitlementAcknowledgementOutbox) Claim(ctx context.Context, previousTenant dispatchapp.ID, claimedAt, leaseUntil time.Time) (record dispatchapp.Record, found bool, err error) {
	if s == nil || s.runner == nil || ctx == nil || claimedAt.IsZero() || !leaseUntil.After(claimedAt) {
		return record, false, dispatchapp.ErrDispatchInvalid
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		queries := controlsql.New(db)
		compatibility, compatibilityErr := queries.GetAIEntitlementDispatchCompatibility(txCtx)
		if compatibilityErr != nil {
			return compatibilityErr
		}
		const generation int64 = 1
		if compatibility.MigrationSet != "control_registry" || compatibility.SchemaVersion != 38 || generation < compatibility.MinReaderGeneration || generation > compatibility.MaxReaderGeneration || generation < compatibility.MinWriterGeneration || generation > compatibility.MaxWriterGeneration {
			return errors.New("ai_entitlement_acknowledgement_control_schema_incompatible")
		}
		row, claimErr := queries.ClaimAIEntitlementAcknowledgement(txCtx, controlsql.ClaimAIEntitlementAcknowledgementParams{LeaseUntil: dispatchTime(leaseUntil), ClaimedAt: dispatchTime(claimedAt), PreviousTenantID: dispatchID(previousTenant)})
		if errors.Is(claimErr, pgx.ErrNoRows) {
			return nil
		}
		if claimErr != nil {
			return claimErr
		}
		if !row.ID.Valid || !row.TenantID.Valid || !row.AggregateID.Valid || !row.OccurredAt.Valid {
			return dispatchapp.ErrDispatchInvalid
		}
		record = dispatchapp.Record{ID: dispatchapp.ID(row.ID.Bytes), TenantID: dispatchapp.ID(row.TenantID.Bytes), ActorID: dispatchapp.ID(row.ActorID.Bytes), AggregateID: dispatchapp.ID(row.AggregateID.Bytes), AggregateType: row.AggregateType, AggregateVersion: row.AggregateVersion, EventType: row.EventType, SchemaVersion: row.SchemaVersion, Classification: row.Classification, Payload: append([]byte(nil), row.Payload...), OccurredAt: row.OccurredAt.Time.UTC(), Attempt: row.Attempts}
		found = true
		return nil
	})
	if err != nil {
		return dispatchapp.Record{}, false, err
	}
	return record, found, nil
}

func (s *AIEntitlementAcknowledgementOutbox) MarkPublished(ctx context.Context, id dispatchapp.ID, expectedAttempt int32, at time.Time) error {
	if s == nil || s.runner == nil || ctx == nil || id.IsZero() || expectedAttempt < 1 || at.IsZero() {
		return dispatchapp.ErrDispatchInvalid
	}
	return s.update(ctx, func(txCtx context.Context, queries *controlsql.Queries) (int64, error) {
		return queries.MarkAIEntitlementAcknowledgementPublished(txCtx, controlsql.MarkAIEntitlementAcknowledgementPublishedParams{PublishedAt: dispatchTime(at), ID: dispatchID(id), ExpectedAttempts: expectedAttempt})
	})
}

func (s *AIEntitlementAcknowledgementOutbox) Defer(ctx context.Context, id dispatchapp.ID, expectedAttempt int32, retryAt time.Time) error {
	if s == nil || s.runner == nil || ctx == nil || id.IsZero() || expectedAttempt < 1 || retryAt.IsZero() {
		return dispatchapp.ErrDispatchInvalid
	}
	return s.update(ctx, func(txCtx context.Context, queries *controlsql.Queries) (int64, error) {
		return queries.DeferAIEntitlementAcknowledgement(txCtx, controlsql.DeferAIEntitlementAcknowledgementParams{RetryAt: dispatchTime(retryAt), ID: dispatchID(id), ExpectedAttempts: expectedAttempt})
	})
}

func (s *AIEntitlementAcknowledgementOutbox) Hold(ctx context.Context, hold dispatchapp.Hold) error {
	if s == nil || s.runner == nil || ctx == nil || hold.EventID.IsZero() || hold.TenantID.IsZero() || hold.EnvelopeHash == [32]byte{} || hold.ExpectedAttempt < 1 || hold.ExpectedAttempt > 9 || hold.HeldAt.IsZero() || !validDispatchHold(hold) {
		return dispatchapp.ErrDispatchInvalid
	}
	return s.update(ctx, func(txCtx context.Context, queries *controlsql.Queries) (int64, error) {
		return queries.HoldAIEntitlementAcknowledgement(txCtx, controlsql.HoldAIEntitlementAcknowledgementParams{EnvelopeHash: hold.EnvelopeHash[:], Category: string(hold.Category), ReasonCode: string(hold.Reason), EventAssignmentGeneration: hold.EventGeneration, ObservedAssignmentGeneration: hold.ObservedGeneration, HeldAt: dispatchTime(hold.HeldAt), EventID: dispatchID(hold.EventID), TenantID: dispatchID(hold.TenantID), ExpectedAttempts: hold.ExpectedAttempt})
	})
}

func validDispatchHold(hold dispatchapp.Hold) bool {
	positive := func(value *int64) bool { return value != nil && *value > 0 }
	switch hold.Category {
	case dispatchapp.HoldStaleAssignment:
		return hold.Reason == dispatchapp.ReasonStaleAssignment && positive(hold.EventGeneration) && positive(hold.ObservedGeneration) && *hold.ObservedGeneration > *hold.EventGeneration
	case dispatchapp.HoldIntegrityConflict:
		return hold.Reason == dispatchapp.ReasonGenerationAhead && positive(hold.EventGeneration) && positive(hold.ObservedGeneration) && *hold.EventGeneration > *hold.ObservedGeneration
	case dispatchapp.HoldPoisonEvent:
		return hold.Reason == dispatchapp.ReasonInvalidEnvelope
	case dispatchapp.HoldDeliveryExhausted:
		return hold.ExpectedAttempt == 9 && (hold.Reason == dispatchapp.ReasonRegistryUnavailable || hold.Reason == dispatchapp.ReasonCurrentAssignmentUnavailable || hold.Reason == dispatchapp.ReasonEndpointUnavailable || hold.Reason == dispatchapp.ReasonTenantApplyUnavailable)
	default:
		return false
	}
}

func (s *AIEntitlementAcknowledgementOutbox) update(ctx context.Context, update func(context.Context, *controlsql.Queries) (int64, error)) error {
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		affected, err := update(txCtx, controlsql.New(db))
		if err != nil {
			return err
		}
		if affected != 1 {
			return dispatchapp.ErrDispatchUnavailable
		}
		return nil
	})
}

func dispatchID(id dispatchapp.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: !id.IsZero()}
}
func dispatchTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}

var _ dispatchapp.Outbox = (*AIEntitlementAcknowledgementOutbox)(nil)
