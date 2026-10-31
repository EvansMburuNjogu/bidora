package persistence

import (
	app "bidora/backend/internal/authorization/application"
	qsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"time"
)

type MembershipOffboardingStore struct{ runner writeRunner }

func NewMembershipOffboardingStore(r writeRunner) (*MembershipOffboardingStore, error) {
	if r == nil {
		return nil, errors.New("membership_offboarding_store_invalid")
	}
	return &MembershipOffboardingStore{r}, nil
}
func (s *MembershipOffboardingStore) BeginMembershipOffboarding(ctx context.Context, b app.MembershipOffboardingBegin) (out app.MembershipOffboardingOperation, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		old, e := q.GetMembershipOffboardingByIdempotency(tc, qsql.GetMembershipOffboardingByIdempotencyParams{ActorID: authorizationID(b.ActorID), KeyHash: b.KeyHash[:]})
		if e == nil {
			if !bytes.Equal(old.RequestHash, b.RequestHash[:]) {
				return app.ErrMembershipOffboardingConflict
			}
			v, e := q.GetMembershipOffboarding(tc, old.ID)
			if e == nil {
				out = mapOffboarding(v)
				out.Replay = true
			}
			return e
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		p, e := q.LockMembershipOffboardingParties(tc, qsql.LockMembershipOffboardingPartiesParams{TargetMembershipID: authorizationID(b.TargetMembershipID), SuccessorMembershipID: authorizationID(b.SuccessorMembershipID), TenantID: authorizationID(b.TenantID), ActorID: authorizationID(b.ActorID), ActorGeneration: b.ActorContextGeneration, TargetGeneration: b.ExpectedTargetGeneration})
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrMembershipOffboardingConflict
		}
		if e != nil {
			return e
		}
		if p.OtherAdminCount < 1 {
			return app.ErrMembershipLastAdmin
		}
		if e = q.StartMembershipOffboarding(tc, qsql.StartMembershipOffboardingParams{ID: authorizationID(b.IDs.OperationID), TenantID: authorizationID(b.TenantID), TargetMembershipID: authorizationID(b.TargetMembershipID), SuccessorMembershipID: authorizationID(b.SuccessorMembershipID), ActorID: authorizationID(b.ActorID), Reason: b.Reason, AssignmentGeneration: p.AssignmentGeneration, KeyHash: b.KeyHash[:], RequestHash: b.RequestHash[:], StartedAt: authorizationTime(b.OccurredAt)}); e != nil {
			return e
		}
		n, e := q.SetMembershipOffboardingState(tc, qsql.SetMembershipOffboardingStateParams{At: authorizationTime(b.OccurredAt), ID: authorizationID(b.TargetMembershipID), ExpectedGeneration: b.ExpectedTargetGeneration})
		if e != nil || n != 1 {
			return app.ErrMembershipOffboardingConflict
		}
		if e = insertOffboardingEvidence(tc, q, b.IDs.StartEventID, b.IDs.StartAuditID, b.IDs.StartOutboxID, b.IDs.OperationID, b.ActorID, b.TenantID, b.TargetMembershipID, b.SuccessorMembershipID, b.CorrelationID, 1, "STARTED", b.Reason, b.OccurredAt, b.KeyHash, b.RequestHash); e != nil {
			return e
		}
		v, e := q.GetMembershipOffboarding(tc, authorizationID(b.IDs.OperationID))
		if e == nil {
			out = mapOffboarding(v)
		}
		return e
	})
	return
}
func (s *MembershipOffboardingStore) MarkMembershipTenantTransferred(ctx context.Context, actor app.ID, generation int64, version int64, correlation app.ID, tr app.OwnershipTransferResult, ids app.MembershipOffboardingIDs, at time.Time) (out app.MembershipOffboardingOperation, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		v, e := q.GetMembershipOffboarding(tc, authorizationID(ids.OperationID))
		if e != nil {
			return e
		}
		if app.ID(v.ActorID.Bytes) != actor {
			return app.ErrMembershipOffboardingNotFound
		}
		wc, rc, vc := tr.WorkspaceCount, tr.RequirementCount, tr.ReviewCount
		n, e := q.MarkMembershipTenantTransferred(tc, qsql.MarkMembershipTenantTransferredParams{WorkspaceCount: &wc, RequirementCount: &rc, ReviewCount: &vc, At: authorizationTime(at), ID: authorizationID(ids.OperationID), ExpectedVersion: version})
		if e != nil || n != 1 {
			return app.ErrMembershipOffboardingConflict
		}
		if e = insertOffboardingEvidence(tc, q, ids.TransferredEventID, ids.TransferredAuditID, ids.TransferredOutboxID, ids.OperationID, actor, app.ID(v.TenantID.Bytes), app.ID(v.TargetMembershipID.Bytes), app.ID(v.SuccessorMembershipID.Bytes), correlation, v.Version+1, "TENANT_TRANSFERRED", v.Reason, at, [32]byte{}, [32]byte{}); e != nil {
			return e
		}
		x, e := q.GetMembershipOffboarding(tc, authorizationID(ids.OperationID))
		if e == nil {
			out = mapOffboarding(x)
		}
		return e
	})
	return
}
func (s *MembershipOffboardingStore) FinalizeMembershipOffboarding(ctx context.Context, actor app.ID, generation int64, correlation app.ID, ids app.MembershipOffboardingIDs, at time.Time) (out app.MembershipOffboardingOperation, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		base, e := q.GetMembershipOffboarding(tc, authorizationID(ids.OperationID))
		if e != nil {
			return e
		}
		if base.State == "COMPLETED" {
			out = mapOffboarding(base)
			out.Replay = true
			return nil
		}
		if app.ID(base.ActorID.Bytes) != actor {
			return app.ErrMembershipOffboardingNotFound
		}
		x, e := q.LockMembershipOffboardingForFinalize(tc, qsql.LockMembershipOffboardingForFinalizeParams{ID: authorizationID(ids.OperationID), ExpectedVersion: base.Version})
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrMembershipOffboardingConflict
		}
		if e != nil {
			return e
		}
		if e = q.RevokeMembershipOffboardingGrants(tc, qsql.RevokeMembershipOffboardingGrantsParams{At: authorizationTime(at), MembershipID: x.TargetMembershipID}); e != nil {
			return e
		}
		n, e := q.FinalizeOffboardedMembership(tc, qsql.FinalizeOffboardedMembershipParams{At: authorizationTime(at), ID: x.TargetMembershipID})
		if e != nil || n != 1 {
			return app.ErrMembershipOffboardingConflict
		}
		n, e = q.CompleteMembershipOffboarding(tc, qsql.CompleteMembershipOffboardingParams{At: authorizationTime(at), ID: x.ID, ExpectedVersion: x.Version})
		if e != nil || n != 1 {
			return app.ErrMembershipOffboardingConflict
		}
		if e = insertOffboardingEvidence(tc, q, ids.FinalEventID, ids.FinalAuditID, ids.FinalOutboxID, ids.OperationID, actor, app.ID(x.TenantID.Bytes), app.ID(x.TargetMembershipID.Bytes), app.ID(x.SuccessorMembershipID.Bytes), correlation, x.Version+1, "COMPLETED", x.Reason, at, [32]byte{}, [32]byte{}); e != nil {
			return e
		}
		v, e := q.GetMembershipOffboarding(tc, x.ID)
		if e == nil {
			out = mapOffboarding(v)
		}
		return e
	})
	return
}
func (s *MembershipOffboardingStore) GetMembershipOffboarding(ctx context.Context, actor, tenant, id app.ID, g int64) (out app.MembershipOffboardingOperation, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if e := authorizeManager(tc, q, actor, tenant, g); e != nil {
			return e
		}
		v, e := q.GetMembershipOffboarding(tc, authorizationID(id))
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrMembershipOffboardingNotFound
		}
		if e != nil {
			return e
		}
		if app.ID(v.TenantID.Bytes) != tenant {
			return app.ErrMembershipOffboardingNotFound
		}
		out = mapOffboarding(v)
		return nil
	})
	return
}
func (s *MembershipOffboardingStore) RetargetMembershipOffboarding(ctx context.Context, c app.MembershipOffboardingChange) (out app.MembershipOffboardingOperation, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if replay, e := offboardingChangeReplay(tc, q, c, "RETARGETED"); e != nil {
			return e
		} else if replay != nil {
			out = *replay
			return nil
		}
		if e := authorizeManager(tc, q, c.ActorID, c.TenantID, c.ActorContextGeneration); e != nil {
			return e
		}
		x, e := q.LockMembershipOffboardingForRetarget(tc, qsql.LockMembershipOffboardingForRetargetParams{ID: authorizationID(c.OperationID), ExpectedVersion: c.ExpectedVersion})
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrMembershipOffboardingConflict
		}
		if e != nil {
			return e
		}
		if app.ID(x.TenantID.Bytes) != c.TenantID {
			return app.ErrMembershipOffboardingNotFound
		}
		if _, e = q.ValidateOffboardingSuccessor(tc, qsql.ValidateOffboardingSuccessorParams{SuccessorMembershipID: authorizationID(c.SuccessorMembershipID), TenantID: authorizationID(c.TenantID), TargetMembershipID: x.TargetMembershipID}); e != nil {
			return app.ErrMembershipOffboardingConflict
		}
		n, e := q.RetargetMembershipOffboarding(tc, qsql.RetargetMembershipOffboardingParams{SuccessorMembershipID: authorizationID(c.SuccessorMembershipID), Reason: c.Reason, ID: x.ID, ExpectedVersion: x.Version})
		if e != nil || n != 1 {
			return app.ErrMembershipOffboardingConflict
		}
		if e = insertOffboardingEvidence(tc, q, c.EventID, c.AuditID, c.OutboxID, c.OperationID, c.ActorID, c.TenantID, app.ID(x.TargetMembershipID.Bytes), c.SuccessorMembershipID, c.CorrelationID, x.Version+1, "RETARGETED", c.Reason, c.OccurredAt, c.KeyHash, c.RequestHash); e != nil {
			return e
		}
		v, e := q.GetMembershipOffboarding(tc, x.ID)
		if e == nil {
			out = mapOffboarding(v)
		}
		return e
	})
	return
}
func (s *MembershipOffboardingStore) CancelMembershipOffboarding(ctx context.Context, c app.MembershipOffboardingChange) (out app.MembershipOffboardingOperation, err error) {
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		if replay, e := offboardingChangeReplay(tc, q, c, "CANCELLED"); e != nil {
			return e
		} else if replay != nil {
			out = *replay
			return nil
		}
		if e := authorizeManager(tc, q, c.ActorID, c.TenantID, c.ActorContextGeneration); e != nil {
			return e
		}
		x, e := q.LockMembershipOffboardingForCancel(tc, qsql.LockMembershipOffboardingForCancelParams{ID: authorizationID(c.OperationID), ExpectedVersion: c.ExpectedVersion})
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrMembershipOffboardingConflict
		}
		if e != nil {
			return e
		}
		n, e := q.ReactivateOffboardingMembership(tc, qsql.ReactivateOffboardingMembershipParams{At: authorizationTime(c.OccurredAt), ID: x.TargetMembershipID})
		if e != nil || n != 1 {
			return app.ErrMembershipOffboardingConflict
		}
		n, e = q.CancelMembershipOffboarding(tc, qsql.CancelMembershipOffboardingParams{At: authorizationTime(c.OccurredAt), ID: x.ID, ExpectedVersion: x.Version})
		if e != nil || n != 1 {
			return app.ErrMembershipOffboardingConflict
		}
		if e = insertOffboardingEvidence(tc, q, c.EventID, c.AuditID, c.OutboxID, c.OperationID, c.ActorID, c.TenantID, app.ID(x.TargetMembershipID.Bytes), app.ID(x.SuccessorMembershipID.Bytes), c.CorrelationID, x.Version+1, "CANCELLED", c.Reason, c.OccurredAt, c.KeyHash, c.RequestHash); e != nil {
			return e
		}
		v, e := q.GetMembershipOffboarding(tc, x.ID)
		if e == nil {
			out = mapOffboarding(v)
		}
		return e
	})
	return
}
func offboardingChangeReplay(ctx context.Context, q *qsql.Queries, c app.MembershipOffboardingChange, kind string) (*app.MembershipOffboardingOperation, error) {
	v, e := q.GetMembershipOffboardingEventByIdempotency(ctx, qsql.GetMembershipOffboardingEventByIdempotencyParams{ActorID: authorizationID(c.ActorID), KeyHash: c.KeyHash[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(v.RequestHash, c.RequestHash[:]) || v.EventType != kind || app.ID(v.OperationID.Bytes) != c.OperationID {
		return nil, app.ErrMembershipOffboardingConflict
	}
	row, e := q.GetMembershipOffboarding(ctx, v.OperationID)
	if e != nil {
		return nil, e
	}
	out := mapOffboarding(row)
	out.Replay = true
	return &out, nil
}
func authorizeManager(ctx context.Context, q *qsql.Queries, a, t app.ID, g int64) error {
	_, e := q.AuthorizeMembershipManagement(ctx, qsql.AuthorizeMembershipManagementParams{ActorUserID: authorizationID(a), TenantID: authorizationID(t), ActorContextGeneration: g})
	if errors.Is(e, pgx.ErrNoRows) {
		return app.ErrMembershipOffboardingNotFound
	}
	return e
}
func insertOffboardingEvidence(ctx context.Context, q *qsql.Queries, event, audit, outbox, op, actor, tenant, target, successor, correlation app.ID, version int64, kind, reason string, at time.Time, key, request [32]byte) error {
	if key == ([32]byte{}) {
		key = sha256.Sum256([]byte(fmt.Sprintf("membership-offboarding-internal:%s:%s:%d", op, kind, version)))
	}
	if request == ([32]byte{}) {
		request = sha256.Sum256([]byte(fmt.Sprintf("%s:%d", kind, version)))
	}
	seq := int32(version)
	if e := q.InsertMembershipOffboardingEvent(ctx, qsql.InsertMembershipOffboardingEventParams{ID: authorizationID(event), OperationID: authorizationID(op), Sequence: seq, EventType: kind, ActorID: authorizationID(actor), SuccessorMembershipID: authorizationID(successor), Reason: reason, KeyHash: key[:], RequestHash: request[:], OccurredAt: authorizationTime(at)}); e != nil {
		return e
	}
	if e := q.InsertMembershipOffboardingAudit(ctx, qsql.InsertMembershipOffboardingAuditParams{ID: authorizationID(audit), TenantID: authorizationID(tenant), ActorID: authorizationID(actor), Action: "MembershipOffboarding" + kind, OperationID: authorizationID(op), OccurredAt: authorizationTime(at), CorrelationID: authorizationID(correlation), OperationVersion: version, TargetMembershipID: authorizationID(target), SuccessorMembershipID: authorizationID(successor)}); e != nil {
		return e
	}
	return q.InsertMembershipOffboardingOutbox(ctx, qsql.InsertMembershipOffboardingOutboxParams{ID: authorizationID(outbox), TenantID: authorizationID(tenant), ActorID: authorizationID(actor), OperationID: authorizationID(op), OperationVersion: version, EventType: "membership.offboarding." + strings.ToLower(kind) + ".v1", TargetMembershipID: authorizationID(target), SuccessorMembershipID: authorizationID(successor), OccurredAt: authorizationTime(at)})
}
func mapOffboarding(v qsql.GetMembershipOffboardingRow) app.MembershipOffboardingOperation {
	return app.MembershipOffboardingOperation{ID: app.ID(v.ID.Bytes), TenantID: app.ID(v.TenantID.Bytes), TargetMembershipID: app.ID(v.TargetMembershipID.Bytes), SuccessorMembershipID: app.ID(v.SuccessorMembershipID.Bytes), TargetUserID: app.ID(v.TargetUserID.Bytes), SuccessorUserID: app.ID(v.SuccessorUserID.Bytes), ActorID: app.ID(v.ActorID.Bytes), State: v.State, Version: v.Version, AssignmentGeneration: v.AssignmentGeneration, Reason: v.Reason, WorkspaceCount: value32(v.WorkspaceCount), RequirementCount: value32(v.RequirementCount), ReviewCount: value32(v.ReviewCount), StartedAt: v.StartedAt.Time.UTC(), TenantTransferredAt: authTime(v.TenantTransferredAt), CompletedAt: authTime(v.CompletedAt), CancelledAt: authTime(v.CancelledAt)}
}
func value32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}
func authTime(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	x := v.Time.UTC()
	return &x
}
