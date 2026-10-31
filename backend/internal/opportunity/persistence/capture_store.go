package persistence

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type transactionRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}

type CaptureStore struct {
	runner               transactionRunner
	tenantID             application.ID
	assignmentGeneration int64
}

func NewCaptureStore(runner transactionRunner, tenantID application.ID, assignmentGeneration int64) (*CaptureStore, error) {
	if runner == nil || tenantID.IsZero() || assignmentGeneration < 1 {
		return nil, errors.New("capture_store_invalid")
	}
	return &CaptureStore{runner, tenantID, assignmentGeneration}, nil
}
func (s *CaptureStore) WithinCapture(ctx context.Context, a application.CaptureAuthority, fn func(application.CaptureTx) error) error {
	if ctx == nil || fn == nil {
		return errors.New("capture_transaction_invalid")
	}
	if s == nil || s.runner == nil {
		return errors.New("capture_store_invalid")
	}
	if a.TenantID != s.tenantID || a.AssignmentGeneration != s.assignmentGeneration {
		return errors.New("capture_tenant_lease_mismatch")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("capture_transaction_unavailable")
		}
		return fn(&captureTx{ctx: txCtx, q: capturesql.New(db)})
	})
}

type captureTx struct {
	ctx context.Context
	q   *capturesql.Queries
}

func (t *captureTx) Claim(_ context.Context, id, actor application.ID, keyHash, requestHash [32]byte, at time.Time) (application.CaptureClaim, error) {
	_, err := t.q.ClaimCaptureIdempotency(t.ctx, capturesql.ClaimCaptureIdempotencyParams{ID: dbID(id), ActorID: dbID(actor), KeyHash: keyHash[:], RequestHash: requestHash[:], CreatedAt: dbTime(at)})
	if err == nil {
		return application.CaptureClaim{State: application.IdempotencyNew, RequestHash: requestHash}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return application.CaptureClaim{}, err
	}
	row, err := t.q.GetCaptureIdempotencyForUpdate(t.ctx, capturesql.GetCaptureIdempotencyForUpdateParams{ActorID: dbID(actor), KeyHash: keyHash[:]})
	if err != nil {
		return application.CaptureClaim{}, err
	}
	var stored [32]byte
	if len(row.RequestHash) != len(stored) {
		return application.CaptureClaim{}, errors.New("capture_request_hash_invalid")
	}
	copy(stored[:], row.RequestHash)
	claim := application.CaptureClaim{RequestHash: stored}
	switch row.State {
	case "PENDING":
		claim.State = application.IdempotencyPending
	case "COMPLETED":
		if row.ResponseStatus == nil || *row.ResponseStatus != 201 || row.ResponseVersion == nil || *row.ResponseVersion != 1 || !row.ResponseRef.Valid || !row.CurrentSourceVersionID.Valid || !row.OpportunityCreatedAt.Valid {
			return application.CaptureClaim{}, errors.New("capture_replay_invalid")
		}
		claim.State = application.IdempotencyCompleted
		claim.OpportunityID = appID(row.ResponseRef)
		claim.SourceVersionID = appID(row.CurrentSourceVersionID)
		claim.CreatedAt = row.OpportunityCreatedAt.Time.UTC()
	default:
		return application.CaptureClaim{}, errors.New("capture_idempotency_state_invalid")
	}
	return claim, nil
}
func (t *captureTx) InsertOpportunity(_ context.Context, opportunityID, sourceID, actorID application.ID, command application.CaptureCommand, at time.Time) error {
	record, ok := command.Record()
	if !ok {
		return errors.New("capture_record_invalid")
	}
	if err := t.q.InsertCapturedOpportunity(t.ctx, capturesql.InsertCapturedOpportunityParams{ID: dbID(opportunityID), CreatedAt: dbTime(at), CreatedBy: dbID(actorID)}); err != nil {
		return err
	}
	var documentID pgtype.UUID
	if record.SourceDocumentVersionID != nil {
		parsed, err := parseUUID(*record.SourceDocumentVersionID)
		if err != nil {
			return err
		}
		documentID = dbID(parsed)
	}
	var closes pgtype.Timestamptz
	if record.ClosesAt != nil {
		closes = dbTime(*record.ClosesAt)
	}
	fingerprint, _ := command.Fingerprint()
	if err := t.q.InsertUnverifiedSourceVersion(t.ctx, capturesql.InsertUnverifiedSourceVersionParams{ID: dbID(sourceID), OpportunityID: dbID(opportunityID), Title: record.Title, BuyerName: record.BuyerName, SourceUri: record.SourceURI, SourceDocumentVersionID: documentID, ClosesAt: closes, SourceTimezone: record.SourceTimezone, SourceDateText: record.SourceDateText, RequestHash: fingerprint[:], CreatedAt: dbTime(at), CreatedBy: dbID(actorID)}); err != nil {
		return err
	}
	rows, err := t.q.SetOpportunityCurrentSource(t.ctx, capturesql.SetOpportunityCurrentSourceParams{SourceVersionID: dbID(sourceID), UpdatedAt: dbTime(at), OpportunityID: dbID(opportunityID)})
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("capture_current_source_conflict")
	}
	return nil
}
func (t *captureTx) AppendAudit(_ context.Context, id, actorID, opportunityID, correlationID application.ID, at time.Time) error {
	summary, err := json.Marshal(struct {
		VerificationState string `json:"verification_state"`
	}{"UNVERIFIED"})
	if err != nil {
		return err
	}
	return t.q.InsertCaptureAudit(t.ctx, capturesql.InsertCaptureAuditParams{ID: dbID(id), ActorID: dbID(actorID), TargetID: dbID(opportunityID), OccurredAt: dbTime(at), CorrelationID: dbID(correlationID), ChangeSummary: summary})
}
func (t *captureTx) AppendOutbox(_ context.Context, id, actorID, opportunityID, sourceID application.ID, at time.Time) error {
	payload, err := json.Marshal(struct {
		OpportunityID   string `json:"opportunity_id"`
		SourceVersionID string `json:"source_version_id"`
	}{formatID(opportunityID), formatID(sourceID)})
	if err != nil {
		return err
	}
	return t.q.InsertCaptureOutbox(t.ctx, capturesql.InsertCaptureOutboxParams{ID: dbID(id), ActorID: dbID(actorID), AggregateID: dbID(opportunityID), Payload: payload, OccurredAt: dbTime(at)})
}
func (t *captureTx) Complete(_ context.Context, id application.ID, requestHash [32]byte, opportunityID application.ID, at time.Time) error {
	rows, err := t.q.CompleteCaptureIdempotency(t.ctx, capturesql.CompleteCaptureIdempotencyParams{ResponseRef: dbID(opportunityID), CompletedAt: dbTime(at), ID: dbID(id), RequestHash: requestHash[:]})
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("capture_idempotency_completion_conflict")
	}
	return nil
}

// Audit deliberately receives no raw source content.
func dbID(id application.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: !id.IsZero()}
}
func appID(id pgtype.UUID) application.ID { return application.ID(id.Bytes) }
func dbTime(at time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: at.UTC(), Valid: !at.IsZero()}
}
func parseUUID(value string) (application.ID, error) {
	raw, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	if err != nil || len(raw) != 16 {
		return application.ID{}, errors.New("capture_document_id_invalid")
	}
	var id application.ID
	copy(id[:], raw)
	return id, nil
}
func formatID(id application.ID) string {
	hexValue := hex.EncodeToString(id[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexValue[:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:])
}
