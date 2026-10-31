// Package application owns the tenant-local durable AI entitlement checkpoint protocol.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

type State string
type Disposition string
type CompletionOperation string

const (
	Pending             State               = "PENDING_RESERVATION"
	Reserved            State               = "RESERVED"
	Denied              State               = "DENIED"
	DispositionReserved Disposition         = "RESERVED"
	DispositionDenied   Disposition         = "DENIED"
	SettlePending       State               = "SETTLE_PENDING"
	ReleasePending      State               = "RELEASE_PENDING"
	Settled             State               = "SETTLED"
	Released            State               = "RELEASED"
	CompletionSettle    CompletionOperation = "SETTLE"
	CompletionRelease   CompletionOperation = "RELEASE"
)

var ErrInvalid = errors.New("ai_entitlement_checkpoint_invalid")
var ErrConflict = errors.New("ai_entitlement_checkpoint_conflict")
var ErrUnavailable = errors.New("ai_entitlement_checkpoint_unavailable")

type Checkpoint struct {
	ID, TenantID, TargetID, SourceOperationID, RequesterUserID, ReservationID ID
	TargetType, UsageKind, DenialCode                                         string
	AssignmentGeneration, EstimatedCredits, Version                           int64
	State                                                                     State
	RequestHash                                                               [32]byte
	CreatedAt, UpdatedAt                                                      time.Time
}
type RequestInput struct {
	TenantID, TargetID, SourceOperationID, RequesterUserID, CorrelationID ID
	TargetType, UsageKind                                                 string
	AssignmentGeneration, EstimatedCredits                                int64
}
type Acknowledgement struct {
	CheckpointID, TenantID, EventID, AggregateID, ReservationID, CorrelationID ID
	Disposition                                                                Disposition
	DenialCode, EventType                                                      string
	AggregateVersion, AssignmentGeneration                                     int64
	EventOccurredAt                                                            time.Time
}
type Result struct {
	Checkpoint Checkpoint
	Replay     bool
}
type RequestCommit struct {
	CheckpointID, AuditID, OutboxID ID
	Input                           RequestInput
	RequestHash                     [32]byte
	OccurredAt                      time.Time
}
type AckCommit struct {
	InboxID, TransitionID, AuditID, OutboxID ID
	Before, After                            Checkpoint
	Acknowledgement                          Acknowledgement
	ProcessedAt                              time.Time
}
type CompletionInput struct {
	CheckpointID, ReservationID, ExecutionID, CorrelationID ID
	Operation                                               CompletionOperation
	ActualCredits                                           int64
}
type CompletionRequest struct {
	ID, CheckpointID, ReservationID, ExecutionID, CorrelationID ID
	Operation                                                   CompletionOperation
	ActualCredits                                               int64
	RequestedAt                                                 time.Time
	Checkpoint                                                  Checkpoint
}
type CompletionRequestCommit struct {
	RequestID, AuditID, OutboxID ID
	Input                        CompletionInput
	Before, After                Checkpoint
	RequestedAt                  time.Time
}
type CompletionAcknowledgementCommit struct {
	AcknowledgementID, AuditID, OutboxID ID
	Request                              CompletionRequest
	Before, After                        Checkpoint
	AcknowledgedAt                       time.Time
}

type Tx interface {
	FindBySource(context.Context, ID) (Checkpoint, bool, error)
	InsertRequest(context.Context, RequestCommit) error
	InboxSeen(context.Context, ID) (bool, error)
	Lock(context.Context, ID) (Checkpoint, bool, error)
	ApplyAcknowledgement(context.Context, AckCommit) error
	FindCompletionRequest(context.Context, ID) (CompletionRequest, bool, error)
	FindPendingCompletion(context.Context) (CompletionRequest, bool, error)
	InsertCompletionRequest(context.Context, CompletionRequestCommit) error
	ApplyCompletionAcknowledgement(context.Context, CompletionAcknowledgementCommit) error
}
type UnitOfWork interface {
	WithinCheckpoint(context.Context, func(context.Context, Tx) error) error
}
type IDGenerator interface{ NewID() (ID, error) }
type Clock interface{ Now() time.Time }
type Service struct {
	uow   UnitOfWork
	ids   IDGenerator
	clock Clock
}

func NewService(uow UnitOfWork, ids IDGenerator, clock Clock) (*Service, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrInvalid
	}
	return &Service{uow, ids, clock}, nil
}
func (s *Service) Request(ctx context.Context, input RequestInput) (result Result, err error) {
	if ctx == nil || !validRequest(input) {
		return result, ErrInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrUnavailable
	}
	ids, err := newIDs(s.ids, 3)
	if err != nil {
		return result, err
	}
	err = s.uow.WithinCheckpoint(ctx, func(txCtx context.Context, tx Tx) error {
		var requestErr error
		result, requestErr = RequestWithinTransaction(txCtx, tx, input, [3]ID{ids[0], ids[1], ids[2]}, at)
		return requestErr
	})
	return result, err
}

// RequestWithinTransaction lets an owning tenant workflow create its target
// linkage and entitlement checkpoint atomically in the same tenant database.
func RequestWithinTransaction(ctx context.Context, tx Tx, input RequestInput, evidenceIDs [3]ID, at time.Time) (result Result, err error) {
	if ctx == nil || tx == nil || !validRequest(input) || at.IsZero() {
		return result, ErrInvalid
	}
	for _, id := range evidenceIDs {
		if id.IsZero() {
			return result, ErrInvalid
		}
	}
	encoded, _ := json.Marshal(struct {
		TenantID, TargetID, SourceOperationID, RequesterUserID ID
		TargetType, UsageKind                                  string
		AssignmentGeneration, EstimatedCredits                 int64
	}{input.TenantID, input.TargetID, input.SourceOperationID, input.RequesterUserID, input.TargetType, input.UsageKind, input.AssignmentGeneration, input.EstimatedCredits})
	hash := sha256.Sum256(append([]byte("tenant-ai-entitlement:v1:"), encoded...))
	existing, found, err := tx.FindBySource(ctx, input.SourceOperationID)
	if err != nil {
		return result, err
	}
	if found {
		if existing.RequestHash != hash {
			return result, ErrConflict
		}
		return Result{Checkpoint: existing, Replay: true}, nil
	}
	commit := RequestCommit{CheckpointID: evidenceIDs[0], AuditID: evidenceIDs[1], OutboxID: evidenceIDs[2], Input: input, RequestHash: hash, OccurredAt: at}
	if err = tx.InsertRequest(ctx, commit); err != nil {
		return result, err
	}
	return Result{Checkpoint: Checkpoint{ID: evidenceIDs[0], TenantID: input.TenantID, TargetID: input.TargetID, SourceOperationID: input.SourceOperationID, RequesterUserID: input.RequesterUserID, TargetType: input.TargetType, UsageKind: input.UsageKind, AssignmentGeneration: input.AssignmentGeneration, EstimatedCredits: input.EstimatedCredits, State: Pending, Version: 1, RequestHash: hash, CreatedAt: at, UpdatedAt: at}}, nil
}
func (s *Service) Acknowledge(ctx context.Context, ack Acknowledgement) (result Result, err error) {
	if ctx == nil || !validAck(ack) {
		return result, ErrInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrUnavailable
	}
	ids, err := newIDs(s.ids, 4)
	if err != nil {
		return result, err
	}
	evidenceIDs := [4]ID{ids[0], ids[1], ids[2], ids[3]}
	err = s.uow.WithinCheckpoint(ctx, func(txCtx context.Context, tx Tx) error {
		var acknowledgeErr error
		result, acknowledgeErr = AcknowledgeWithinTransaction(txCtx, tx, ack, evidenceIDs, at)
		return acknowledgeErr
	})
	return result, err
}

// AcknowledgeWithinTransaction lets an owning workflow atomically apply the
// checkpoint and its local target effect without exposing persistence details.
func AcknowledgeWithinTransaction(ctx context.Context, tx Tx, ack Acknowledgement, evidenceIDs [4]ID, at time.Time) (result Result, err error) {
	if ctx == nil || tx == nil || !validAck(ack) || at.IsZero() {
		return result, ErrInvalid
	}
	for _, id := range evidenceIDs {
		if id.IsZero() {
			return result, ErrInvalid
		}
	}
	seen, err := tx.InboxSeen(ctx, ack.EventID)
	if err != nil {
		return result, err
	}
	current, found, err := tx.Lock(ctx, ack.CheckpointID)
	if err != nil {
		return result, err
	}
	if !found || current.TenantID != ack.TenantID || current.AssignmentGeneration != ack.AssignmentGeneration {
		return result, ErrConflict
	}
	if seen {
		return Result{Checkpoint: current, Replay: true}, nil
	}
	if current.State != Pending || current.Version != 1 {
		return result, ErrConflict
	}
	after := current
	after.Version = 2
	after.UpdatedAt = at.UTC()
	if ack.Disposition == DispositionReserved {
		if ack.AggregateID != ack.ReservationID {
			return result, ErrInvalid
		}
		after.State = Reserved
		after.ReservationID = ack.ReservationID
	} else {
		if ack.AggregateID != ack.CheckpointID {
			return result, ErrInvalid
		}
		after.State = Denied
		after.DenialCode = ack.DenialCode
	}
	commit := AckCommit{InboxID: evidenceIDs[0], TransitionID: evidenceIDs[1], AuditID: evidenceIDs[2], OutboxID: evidenceIDs[3], Before: current, After: after, Acknowledgement: ack, ProcessedAt: at.UTC()}
	if err = tx.ApplyAcknowledgement(ctx, commit); err != nil {
		return result, err
	}
	return Result{Checkpoint: after}, nil
}
func RequestCompletionWithinTransaction(ctx context.Context, tx Tx, input CompletionInput, evidenceIDs [3]ID, at time.Time) (result CompletionRequest, replay bool, err error) {
	if ctx == nil || tx == nil || !validCompletionInput(input) || at.IsZero() {
		return result, false, ErrInvalid
	}
	for _, id := range evidenceIDs {
		if id.IsZero() {
			return result, false, ErrInvalid
		}
	}
	existing, found, err := tx.FindCompletionRequest(ctx, input.CheckpointID)
	if err != nil {
		return result, false, err
	}
	if found {
		if !sameCompletion(existing, input) {
			return result, false, ErrConflict
		}
		return existing, true, nil
	}
	before, found, err := tx.Lock(ctx, input.CheckpointID)
	if err != nil {
		return result, false, err
	}
	if !found || before.State != Reserved || before.Version != 2 || before.ReservationID != input.ReservationID {
		return result, false, ErrConflict
	}
	after := before
	after.Version = 3
	after.UpdatedAt = at.UTC()
	if input.Operation == CompletionSettle {
		after.State = SettlePending
	} else {
		after.State = ReleasePending
	}
	commit := CompletionRequestCommit{RequestID: evidenceIDs[0], AuditID: evidenceIDs[1], OutboxID: evidenceIDs[2], Input: input, Before: before, After: after, RequestedAt: at.UTC()}
	if err = tx.InsertCompletionRequest(ctx, commit); err != nil {
		return result, false, err
	}
	return CompletionRequest{ID: evidenceIDs[0], CheckpointID: input.CheckpointID, ReservationID: input.ReservationID, ExecutionID: input.ExecutionID, CorrelationID: input.CorrelationID, Operation: input.Operation, ActualCredits: input.ActualCredits, RequestedAt: at.UTC(), Checkpoint: after}, false, nil
}

func AcknowledgeCompletionWithinTransaction(ctx context.Context, tx Tx, request CompletionRequest, evidenceIDs [3]ID, at time.Time) (Checkpoint, bool, error) {
	if ctx == nil || tx == nil || !validCompletionRequest(request) || at.IsZero() {
		return Checkpoint{}, false, ErrInvalid
	}
	for _, id := range evidenceIDs {
		if id.IsZero() {
			return Checkpoint{}, false, ErrInvalid
		}
	}
	current, found, err := tx.Lock(ctx, request.CheckpointID)
	if err != nil {
		return Checkpoint{}, false, err
	}
	if !found || current.ReservationID != request.ReservationID {
		return Checkpoint{}, false, ErrConflict
	}
	desired := Settled
	expected := SettlePending
	if request.Operation == CompletionRelease {
		desired, expected = Released, ReleasePending
	}
	if current.State == desired && current.Version == 4 {
		return current, true, nil
	}
	if current.State != expected || current.Version != 3 {
		return Checkpoint{}, false, ErrConflict
	}
	after := current
	after.State, after.Version, after.UpdatedAt = desired, 4, at.UTC()
	commit := CompletionAcknowledgementCommit{AcknowledgementID: evidenceIDs[0], AuditID: evidenceIDs[1], OutboxID: evidenceIDs[2], Request: request, Before: current, After: after, AcknowledgedAt: at.UTC()}
	if err = tx.ApplyCompletionAcknowledgement(ctx, commit); err != nil {
		return Checkpoint{}, false, err
	}
	return after, false, nil
}

func validCompletionInput(v CompletionInput) bool {
	if v.CheckpointID.IsZero() || v.ReservationID.IsZero() || v.ExecutionID.IsZero() || v.CorrelationID.IsZero() {
		return false
	}
	return (v.Operation == CompletionSettle && v.ActualCredits > 0) || (v.Operation == CompletionRelease && v.ActualCredits == 0)
}
func validCompletionRequest(v CompletionRequest) bool {
	return !v.ID.IsZero() && !v.RequestedAt.IsZero() && validCompletionInput(CompletionInput{CheckpointID: v.CheckpointID, ReservationID: v.ReservationID, ExecutionID: v.ExecutionID, CorrelationID: v.CorrelationID, Operation: v.Operation, ActualCredits: v.ActualCredits})
}
func sameCompletion(existing CompletionRequest, input CompletionInput) bool {
	return existing.CheckpointID == input.CheckpointID && existing.ReservationID == input.ReservationID && existing.ExecutionID == input.ExecutionID && existing.CorrelationID == input.CorrelationID && existing.Operation == input.Operation && existing.ActualCredits == input.ActualCredits
}

func validRequest(v RequestInput) bool {
	return !v.TenantID.IsZero() && !v.TargetID.IsZero() && !v.SourceOperationID.IsZero() && !v.RequesterUserID.IsZero() && !v.CorrelationID.IsZero() && v.AssignmentGeneration > 0 && v.EstimatedCredits > 0 && validTarget(v.TargetType) && validKind(v.UsageKind)
}
func validAck(v Acknowledgement) bool {
	if v.CheckpointID.IsZero() || v.TenantID.IsZero() || v.EventID.IsZero() || v.AggregateID.IsZero() || v.CorrelationID.IsZero() || v.AggregateVersion != 1 || v.AssignmentGeneration < 1 || v.EventOccurredAt.IsZero() {
		return false
	}
	if v.Disposition == DispositionReserved {
		return v.EventType == "commercial.ai_usage.reserved.v1" && !v.ReservationID.IsZero() && v.DenialCode == ""
	}
	return v.Disposition == DispositionDenied && v.EventType == "commercial.ai_usage.denied.v1" && v.ReservationID.IsZero() && (v.DenialCode == "LIMIT_EXCEEDED" || v.DenialCode == "ACCESS_UNAVAILABLE" || v.DenialCode == "SUBSCRIPTION_UNAVAILABLE")
}
func validTarget(v string) bool {
	return v == "TENDER_ANALYSIS" || v == "REQUIREMENT_REVIEW" || v == "RESPONSE_DRAFT" || v == "PROPOSAL_DRAFT"
}
func validKind(v string) bool {
	return v == "DOCUMENT_PDF" || v == "DOCUMENT_DOCX" || v == "GENERATION_TEXT"
}
func newIDs(g IDGenerator, n int) ([]ID, error) {
	out := make([]ID, n)
	for i := range out {
		id, e := g.NewID()
		if e != nil || id.IsZero() {
			return nil, ErrUnavailable
		}
		out[i] = id
	}
	return out, nil
}
