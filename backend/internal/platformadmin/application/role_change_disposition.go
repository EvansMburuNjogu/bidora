package application

import (
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"strings"
	"time"

	"bidora/backend/internal/platformadmin/domain"
)

type OwnerRoleDispositionInput struct {
	CheckerID, SessionID, ChangeID, CorrelationID, IdempotencyKey ID
	PlatformContextGeneration, ExpectedChangeVersion              int64
	Reason                                                        string
}
type OwnerRoleDispositionResult struct {
	ChangeID, TargetUserID, AssignmentID ID
	Action                               RoleChangeAction
	State                                string
	RevokedSessions                      int64
	Replay                               bool
}

func (s *RoleChangeService) ApproveOwnerRoleChange(ctx context.Context, input OwnerRoleDispositionInput) (result OwnerRoleDispositionResult, err error) {
	reason, err := validateOwnerRoleDisposition(ctx, s, input)
	if err != nil {
		return result, err
	}
	now := s.clock.Now().UTC()
	keyHash := hashKey("platform-owner-role-approve:v1", input.IdempotencyKey)
	requestHash := hashRequest("platform-owner-role-approve:v1", struct {
		Change  ID
		Version int64
		Reason  string
	}{input.ChangeID, input.ExpectedChangeVersion, reason})
	var notification *PlatformSecurityDeliveryRequest
	err = s.uow.WithinRoleChange(ctx, func(txCtx context.Context, tx RoleChangeTx) error {
		replay, found, replayErr := tx.GetCommand(txCtx, input.CheckerID, "ADMINISTRATOR_CHANGE_APPROVE", keyHash)
		if replayErr != nil {
			return replayErr
		}
		if found {
			if replay.RequestHash != requestHash || replay.TargetID != input.ChangeID {
				return ErrInvitationConflict
			}
			result = OwnerRoleDispositionResult{ChangeID: input.ChangeID, State: "EXECUTED", Replay: true}
			return nil
		}
		checker, _, authErr := requireRoleChangeChecker(txCtx, tx, input, now)
		if authErr != nil {
			return authErr
		}
		governance, lockErr := tx.LockGovernance(txCtx)
		if lockErr != nil {
			return lockErr
		}
		pending, lockErr := tx.LockPendingOwnerRoleChange(txCtx, input.ChangeID, now)
		if lockErr != nil {
			return lockErr
		}
		if pending.Version != input.ExpectedChangeVersion {
			return ErrConcurrentChange
		}
		if checker.UserID == pending.MakerID || checker.UserID == pending.TargetUserID {
			return ErrPlatformAccessForbidden
		}
		ownerIDs, lockErr := tx.LockActiveOwnerIDs(txCtx)
		if lockErr != nil {
			return lockErr
		}
		effective, lockErr := tx.CountEffectiveOwnerIdentities(txCtx, ownerIDs)
		if lockErr != nil {
			return lockErr
		}
		if effective < 2 || !containsID(ownerIDs, pending.MakerID) {
			return ErrInvitationConflict
		}
		makerEffective, lockErr := tx.CountEffectiveOwnerIdentities(txCtx, []ID{pending.MakerID})
		if lockErr != nil {
			return lockErr
		}
		if makerEffective != 1 {
			return ErrPlatformAccessForbidden
		}
		profile, lockErr := tx.LockAdministratorProfile(txCtx, pending.TargetUserID)
		if lockErr != nil {
			return lockErr
		}
		if profile.Version != pending.ExpectedAdministratorVersion || profile.ContextGeneration != pending.ExpectedContextGeneration {
			return ErrConcurrentChange
		}
		identity, lockErr := tx.LockAdministratorIdentity(txCtx, pending.TargetUserID)
		if lockErr != nil {
			return lockErr
		}
		ids, lockErr := newRoleChangeIDs(s.ids, 4)
		if lockErr != nil {
			return lockErr
		}
		assignmentID := pending.AssignmentID
		if pending.Action == AssignRole {
			if identity.Disabled {
				return ErrPlatformAccessForbidden
			}
			exists, existsErr := tx.ActiveRoleExists(txCtx, pending.TargetUserID, domain.PlatformOwner)
			if existsErr != nil {
				return existsErr
			}
			if exists {
				return ErrConcurrentChange
			}
			assignmentID = ids[0]
			if lockErr = tx.InsertRoleAssignment(txCtx, assignmentID, pending.TargetUserID, domain.PlatformOwner, pending.MakerID, pending.Reason, now); lockErr != nil {
				return lockErr
			}
		} else {
			assignment, assignmentErr := tx.LockRoleAssignment(txCtx, pending.AssignmentID, pending.TargetUserID)
			if assignmentErr != nil {
				return assignmentErr
			}
			if assignment.Role != domain.PlatformOwner || assignment.Version != pending.ExpectedAssignmentVersion {
				return ErrConcurrentChange
			}
			targetEffective, countErr := tx.CountEffectiveOwnerIdentities(txCtx, []ID{pending.TargetUserID})
			if countErr != nil {
				return countErr
			}
			if targetEffective != 1 || effective <= 1 {
				return ErrInvitationConflict
			}
			if lockErr = tx.RevokeRoleAssignment(txCtx, assignment, pending.MakerID, pending.Reason, now); lockErr != nil {
				return lockErr
			}
		}
		if lockErr = tx.AdvanceAdministratorProfile(txCtx, profile, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.AdvanceIdentityEpoch(txCtx, identity, now); lockErr != nil {
			return lockErr
		}
		revoked, lockErr := tx.RevokeSessions(txCtx, identity.UserID, now)
		if lockErr != nil {
			return lockErr
		}
		if lockErr = tx.AdvanceOwnerGeneration(txCtx, governance.Version, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.DecideOwnerRoleChange(txCtx, pending.ChangeID, pending.Version, checker.UserID, "EXECUTED", reason, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.ClaimCommand(txCtx, ids[1], checker.UserID, "ADMINISTRATOR_CHANGE_APPROVE", keyHash, requestHash, pending.ChangeID, 200, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.AppendRoleChangeEvidence(txCtx, RoleChangeEvidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: input.CorrelationID, ActorID: checker.UserID, TargetUserID: pending.TargetUserID, ChangeID: pending.ChangeID, Action: pending.Action, Role: domain.PlatformOwner, State: "EXECUTED", Version: pending.Version + 1, OccurredAt: now}); lockErr != nil {
			return lockErr
		}
		result = OwnerRoleDispositionResult{ChangeID: pending.ChangeID, TargetUserID: pending.TargetUserID, AssignmentID: assignmentID, Action: pending.Action, State: "EXECUTED", RevokedSessions: revoked}
		notification = securityDeliveryRequest(pending.ChangeID, checker.UserID, identity, input.CorrelationID, now)
		return nil
	})
	if err == nil && notification != nil && s.delivery != nil {
		_ = s.delivery.DeliverPlatformRoleChange(ctx, *notification, domain.PlatformOwner, result.Action == AssignRole)
	}
	return result, err
}

func (s *RoleChangeService) RejectOwnerRoleChange(ctx context.Context, input OwnerRoleDispositionInput) (result OwnerRoleDispositionResult, err error) {
	reason, err := validateOwnerRoleDisposition(ctx, s, input)
	if err != nil {
		return result, err
	}
	now := s.clock.Now().UTC()
	keyHash := hashKey("platform-owner-role-reject:v1", input.IdempotencyKey)
	requestHash := hashRequest("platform-owner-role-reject:v1", struct {
		Change  ID
		Version int64
		Reason  string
	}{input.ChangeID, input.ExpectedChangeVersion, reason})
	err = s.uow.WithinRoleChange(ctx, func(txCtx context.Context, tx RoleChangeTx) error {
		replay, found, replayErr := tx.GetCommand(txCtx, input.CheckerID, "ADMINISTRATOR_CHANGE_REJECT", keyHash)
		if replayErr != nil {
			return replayErr
		}
		if found {
			if replay.RequestHash != requestHash || replay.TargetID != input.ChangeID {
				return ErrInvitationConflict
			}
			result = OwnerRoleDispositionResult{ChangeID: input.ChangeID, State: "REJECTED", Replay: true}
			return nil
		}
		checker, _, authErr := requireRoleChangeChecker(txCtx, tx, input, now)
		if authErr != nil {
			return authErr
		}
		if _, lockErr := tx.LockGovernance(txCtx); lockErr != nil {
			return lockErr
		}
		pending, lockErr := tx.LockPendingOwnerRoleChange(txCtx, input.ChangeID, now)
		if lockErr != nil {
			return lockErr
		}
		if pending.Version != input.ExpectedChangeVersion {
			return ErrConcurrentChange
		}
		if checker.UserID == pending.MakerID || checker.UserID == pending.TargetUserID {
			return ErrPlatformAccessForbidden
		}
		ownerIDs, lockErr := tx.LockActiveOwnerIDs(txCtx)
		if lockErr != nil {
			return lockErr
		}
		effective, lockErr := tx.CountEffectiveOwnerIdentities(txCtx, ownerIDs)
		if lockErr != nil {
			return lockErr
		}
		if effective < 2 || !containsID(ownerIDs, pending.MakerID) {
			return ErrInvitationConflict
		}
		makerEffective, lockErr := tx.CountEffectiveOwnerIdentities(txCtx, []ID{pending.MakerID})
		if lockErr != nil {
			return lockErr
		}
		if makerEffective != 1 {
			return ErrPlatformAccessForbidden
		}
		ids, lockErr := newRoleChangeIDs(s.ids, 3)
		if lockErr != nil {
			return lockErr
		}
		if lockErr = tx.DecideOwnerRoleChange(txCtx, pending.ChangeID, pending.Version, checker.UserID, "REJECTED", reason, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.ClaimCommand(txCtx, ids[0], checker.UserID, "ADMINISTRATOR_CHANGE_REJECT", keyHash, requestHash, pending.ChangeID, 200, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.AppendRoleChangeEvidence(txCtx, RoleChangeEvidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: input.CorrelationID, ActorID: checker.UserID, TargetUserID: pending.TargetUserID, ChangeID: pending.ChangeID, Action: pending.Action, Role: domain.PlatformOwner, State: "REJECTED", Version: pending.Version + 1, OccurredAt: now}); lockErr != nil {
			return lockErr
		}
		result = OwnerRoleDispositionResult{ChangeID: pending.ChangeID, TargetUserID: pending.TargetUserID, AssignmentID: pending.AssignmentID, Action: pending.Action, State: "REJECTED"}
		return nil
	})
	return result, err
}

func validateOwnerRoleDisposition(ctx context.Context, s *RoleChangeService, input OwnerRoleDispositionInput) (string, error) {
	reason := strings.TrimSpace(input.Reason)
	if ctx == nil || s == nil || s.uow == nil || s.ids == nil || s.clock == nil || input.CheckerID.IsZero() || input.SessionID.IsZero() || input.ChangeID.IsZero() || input.CorrelationID.IsZero() || input.IdempotencyKey.IsZero() || input.PlatformContextGeneration < 1 || input.ExpectedChangeVersion < 1 || reason != input.Reason || len(reason) < 1 || len(reason) > 2000 {
		return "", ErrInvitationInvalid
	}
	return reason, nil
}
func requireRoleChangeChecker(ctx context.Context, tx RoleChangeTx, input OwnerRoleDispositionInput, now time.Time) (AuthorityFacts, IdentityAuthorityFacts, error) {
	facts, identity, err := resolveAuthorityDetails(ctx, tx, AuthorityQuery{UserID: input.CheckerID, SessionID: input.SessionID, ObservedAt: now})
	if err != nil {
		return AuthorityFacts{}, IdentityAuthorityFacts{}, ErrPlatformAccessForbidden
	}
	if facts.ContextGeneration != input.PlatformContextGeneration {
		return AuthorityFacts{}, IdentityAuthorityFacts{}, ErrPlatformContextStale
	}
	eligible := containsRole(facts.Roles, domain.PlatformOwner) && facts.Grants("platform.governance.approve") || containsRole(facts.Roles, domain.SecurityAuditAdministrator) && facts.Grants("platform.governance.review")
	if !eligible {
		return AuthorityFacts{}, IdentityAuthorityFacts{}, ErrPlatformAccessForbidden
	}
	if !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.After(now) || facts.AssuredAt.Before(now.Add(-recentPlatformAssurance)) {
		return AuthorityFacts{}, IdentityAuthorityFacts{}, ErrPlatformAssuranceRequired
	}
	return facts, identity, nil
}
