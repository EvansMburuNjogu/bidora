package application

import (
	"bidora/backend/internal/identity/assurance"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrMembershipOffboardingConflict = errors.New("membership_offboarding_conflict")
var ErrMembershipOffboardingNotFound = errors.New("membership_offboarding_not_found")
var ErrMembershipLastAdmin = errors.New("membership_last_admin")
var ErrMembershipTransferLimit = errors.New("membership_transfer_limit")

type MembershipOffboardingInput struct {
	ActorID, TenantID, TargetMembershipID, SuccessorMembershipID, CorrelationID ID
	ActorContextGeneration, ExpectedTargetGeneration                            int64
	Reason, IdempotencyKey                                                      string
	SessionAuthenticatedAt, AssuranceAt                                         time.Time
	AssuranceLevel, AssuranceMethod                                             string
}
type MembershipOffboardingOperation struct {
	ID, TenantID, TargetMembershipID, SuccessorMembershipID, TargetUserID, SuccessorUserID, ActorID ID
	State                                                                                           string
	Version, AssignmentGeneration                                                                   int64
	Reason                                                                                          string
	WorkspaceCount, RequirementCount, ReviewCount                                                   int32
	StartedAt                                                                                       time.Time
	TenantTransferredAt, CompletedAt, CancelledAt                                                   *time.Time
	Replay                                                                                          bool
}
type MembershipOffboardingIDs struct{ OperationID, StartEventID, StartAuditID, StartOutboxID, TransferEventID, TransferAuditID, TransferOutboxID, TransferredEventID, TransferredAuditID, TransferredOutboxID, FinalEventID, FinalAuditID, FinalOutboxID ID }
type MembershipOffboardingBegin struct {
	MembershipOffboardingInput
	IDs                  MembershipOffboardingIDs
	KeyHash, RequestHash [32]byte
	OccurredAt           time.Time
}
type OwnershipTransferResult struct {
	WorkspaceCount, RequirementCount, ReviewCount int32
	OccurredAt                                    time.Time
	Replay                                        bool
}
type MembershipOffboardingControl interface {
	BeginMembershipOffboarding(context.Context, MembershipOffboardingBegin) (MembershipOffboardingOperation, error)
	MarkMembershipTenantTransferred(context.Context, ID, int64, int64, ID, OwnershipTransferResult, MembershipOffboardingIDs, time.Time) (MembershipOffboardingOperation, error)
	FinalizeMembershipOffboarding(context.Context, ID, int64, ID, MembershipOffboardingIDs, time.Time) (MembershipOffboardingOperation, error)
	GetMembershipOffboarding(context.Context, ID, ID, ID, int64) (MembershipOffboardingOperation, error)
	RetargetMembershipOffboarding(context.Context, MembershipOffboardingChange) (MembershipOffboardingOperation, error)
	CancelMembershipOffboarding(context.Context, MembershipOffboardingChange) (MembershipOffboardingOperation, error)
}
type MembershipOwnershipTransfer interface {
	TransferMembershipOwnership(context.Context, MembershipOffboardingOperation, ID, string, ID, MembershipOffboardingIDs, time.Time) (OwnershipTransferResult, error)
	ClearMembershipOwnershipFence(context.Context, MembershipOffboardingOperation, time.Time) error
}
type MembershipOffboardingChange struct {
	ActorID, TenantID, OperationID, SuccessorMembershipID, CorrelationID ID
	ActorContextGeneration, ExpectedVersion                              int64
	Reason, IdempotencyKey                                               string
	SessionAuthenticatedAt, AssuranceAt, OccurredAt                      time.Time
	AssuranceLevel, AssuranceMethod                                      string
	EventID, AuditID, OutboxID                                           ID
	KeyHash, RequestHash                                                 [32]byte
}
type MembershipOffboardingService struct {
	control MembershipOffboardingControl
	tenant  MembershipOwnershipTransfer
	ids     InvitationIDGenerator
	clock   InvitationClock
}

func NewMembershipOffboardingService(c MembershipOffboardingControl, t MembershipOwnershipTransfer, ids InvitationIDGenerator, clock InvitationClock) (*MembershipOffboardingService, error) {
	if c == nil || t == nil || ids == nil || clock == nil {
		return nil, errors.New("membership_offboarding_service_invalid")
	}
	return &MembershipOffboardingService{c, t, ids, clock}, nil
}
func (s *MembershipOffboardingService) Begin(ctx context.Context, in MembershipOffboardingInput) (MembershipOffboardingOperation, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	now := s.clock.Now().UTC()
	if !validOffboarding(ctx, in.ActorID, in.TenantID, in.CorrelationID, in.ActorContextGeneration, in.Reason, in.IdempotencyKey, in.SessionAuthenticatedAt, in.AssuranceLevel, in.AssuranceMethod, in.AssuranceAt, now) || in.TargetMembershipID.IsZero() || in.SuccessorMembershipID.IsZero() || in.TargetMembershipID == in.SuccessorMembershipID || in.ExpectedTargetGeneration < 1 {
		return MembershipOffboardingOperation{}, safe("membership_offboarding_invalid", nil)
	}
	ids, e := s.newIDs(13)
	if e != nil {
		return MembershipOffboardingOperation{}, safe("membership_offboarding_unavailable", e)
	}
	raw, _ := json.Marshal(struct {
		Tenant, Target, Successor ID
		Generation                int64
		Reason                    string
	}{in.TenantID, in.TargetMembershipID, in.SuccessorMembershipID, in.ExpectedTargetGeneration, in.Reason})
	begin := MembershipOffboardingBegin{MembershipOffboardingInput: in, IDs: ids, KeyHash: sha256.Sum256([]byte("membership-offboarding:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(raw), OccurredAt: now}
	op, e := s.control.BeginMembershipOffboarding(ctx, begin)
	if e != nil {
		return MembershipOffboardingOperation{}, offboardingError(e)
	}
	if op.State == "COMPLETED" || op.State == "CANCELLED" {
		return op, nil
	}
	ids.OperationID = op.ID
	if op.State == "STARTED" {
		tr, e := s.tenant.TransferMembershipOwnership(ctx, op, in.ActorID, in.Reason, in.CorrelationID, ids, now)
		if e != nil {
			return op, offboardingError(e)
		}
		op, e = s.control.MarkMembershipTenantTransferred(ctx, in.ActorID, in.ActorContextGeneration, op.Version, in.CorrelationID, tr, ids, now)
		if e != nil {
			return op, offboardingError(e)
		}
	}
	if op.State == "TENANT_TRANSFERRED" {
		op, e = s.control.FinalizeMembershipOffboarding(ctx, in.ActorID, in.ActorContextGeneration, in.CorrelationID, ids, now)
		if e != nil {
			return op, offboardingError(e)
		}
	}
	return op, nil
}
func (s *MembershipOffboardingService) Get(ctx context.Context, actor, tenant, op ID, generation int64) (MembershipOffboardingOperation, error) {
	if ctx == nil || actor.IsZero() || tenant.IsZero() || op.IsZero() || generation < 1 {
		return MembershipOffboardingOperation{}, safe("membership_offboarding_invalid", nil)
	}
	v, e := s.control.GetMembershipOffboarding(ctx, actor, tenant, op, generation)
	if e != nil {
		return v, offboardingError(e)
	}
	return v, nil
}
func (s *MembershipOffboardingService) Retarget(ctx context.Context, c MembershipOffboardingChange) (MembershipOffboardingOperation, error) {
	now := s.clock.Now().UTC()
	c.Reason = strings.TrimSpace(c.Reason)
	if !validOffboarding(ctx, c.ActorID, c.TenantID, c.CorrelationID, c.ActorContextGeneration, c.Reason, c.IdempotencyKey, c.SessionAuthenticatedAt, c.AssuranceLevel, c.AssuranceMethod, c.AssuranceAt, now) || c.OperationID.IsZero() || c.SuccessorMembershipID.IsZero() || c.ExpectedVersion < 1 {
		return MembershipOffboardingOperation{}, safe("membership_offboarding_invalid", nil)
	}
	ids, e := s.newIDs(3)
	if e != nil {
		return MembershipOffboardingOperation{}, safe("membership_offboarding_unavailable", e)
	}
	c.EventID, c.AuditID, c.OutboxID = ids.OperationID, ids.StartEventID, ids.StartAuditID
	c.OccurredAt = now
	c.KeyHash = sha256.Sum256([]byte("membership-offboarding-retarget:v1:" + c.IdempotencyKey))
	raw, _ := json.Marshal(struct {
		Operation, Successor ID
		Version              int64
		Reason               string
	}{c.OperationID, c.SuccessorMembershipID, c.ExpectedVersion, c.Reason})
	c.RequestHash = sha256.Sum256(raw)
	v, e := s.control.RetargetMembershipOffboarding(ctx, c)
	if e != nil {
		return v, offboardingError(e)
	}
	return v, nil
}
func (s *MembershipOffboardingService) Cancel(ctx context.Context, c MembershipOffboardingChange) (MembershipOffboardingOperation, error) {
	now := s.clock.Now().UTC()
	c.Reason = strings.TrimSpace(c.Reason)
	if !validOffboarding(ctx, c.ActorID, c.TenantID, c.CorrelationID, c.ActorContextGeneration, c.Reason, c.IdempotencyKey, c.SessionAuthenticatedAt, c.AssuranceLevel, c.AssuranceMethod, c.AssuranceAt, now) || c.OperationID.IsZero() || c.ExpectedVersion < 1 {
		return MembershipOffboardingOperation{}, safe("membership_offboarding_invalid", nil)
	}
	ids, e := s.newIDs(3)
	if e != nil {
		return MembershipOffboardingOperation{}, safe("membership_offboarding_unavailable", e)
	}
	c.EventID, c.AuditID, c.OutboxID = ids.OperationID, ids.StartEventID, ids.StartAuditID
	c.OccurredAt = now
	c.KeyHash = sha256.Sum256([]byte("membership-offboarding-cancel:v1:" + c.IdempotencyKey))
	raw, _ := json.Marshal(struct {
		Operation ID
		Version   int64
		Reason    string
	}{c.OperationID, c.ExpectedVersion, c.Reason})
	c.RequestHash = sha256.Sum256(raw)
	v, e := s.control.CancelMembershipOffboarding(ctx, c)
	if e != nil {
		return v, offboardingError(e)
	}
	if e = s.tenant.ClearMembershipOwnershipFence(ctx, v, now); e != nil {
		return v, offboardingError(e)
	}
	return v, nil
}
func validOffboarding(ctx context.Context, a, t, c ID, g int64, r, k string, auth time.Time, level, method string, assured, now time.Time) bool {
	authAge, assuranceAge := now.Sub(auth.UTC()), now.Sub(assured.UTC())
	return ctx != nil && !a.IsZero() && !t.IsZero() && !c.IsZero() && g > 0 && len(r) >= 1 && len(r) <= 2000 && len(k) >= 1 && len(k) <= 200 && !auth.IsZero() && !now.IsZero() && authAge >= 0 && authAge <= 15*time.Minute && assurance.Interactive(level, method) && assuranceAge >= 0 && assuranceAge <= 15*time.Minute
}
func (s *MembershipOffboardingService) newIDs(n int) (MembershipOffboardingIDs, error) {
	v := make([]ID, n)
	for i := range v {
		var e error
		v[i], e = s.ids.NewID()
		if e != nil || v[i].IsZero() {
			return MembershipOffboardingIDs{}, e
		}
	}
	p := MembershipOffboardingIDs{}
	d := []*ID{&p.OperationID, &p.StartEventID, &p.StartAuditID, &p.StartOutboxID, &p.TransferEventID, &p.TransferAuditID, &p.TransferOutboxID, &p.TransferredEventID, &p.TransferredAuditID, &p.TransferredOutboxID, &p.FinalEventID, &p.FinalAuditID, &p.FinalOutboxID}
	for i := range v {
		*d[i] = v[i]
	}
	return p, nil
}
func offboardingError(e error) error {
	switch {
	case errors.Is(e, ErrMembershipLastAdmin):
		return safe("membership_last_admin", e)
	case errors.Is(e, ErrMembershipTransferLimit):
		return safe("membership_transfer_limit", e)
	case errors.Is(e, ErrMembershipOffboardingConflict):
		return safe("membership_offboarding_conflict", e)
	case errors.Is(e, ErrMembershipOffboardingNotFound):
		return safe("membership_offboarding_not_found", e)
	default:
		return safe("membership_offboarding_unavailable", e)
	}
}
