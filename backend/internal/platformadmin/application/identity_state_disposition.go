package application

import (
	"context"

	"bidora/backend/internal/platformadmin/domain"
)

type OwnerIdentityDispositionInput = OwnerRoleDispositionInput
type OwnerIdentityDispositionResult struct {
	ChangeID, TargetUserID ID
	Action                 IdentityStateAction
	State                  string
	RevokedSessions        int64
	Replay                 bool
}

func (s *RoleChangeService) ApproveOwnerIdentityStateChange(ctx context.Context, input OwnerIdentityDispositionInput) (result OwnerIdentityDispositionResult, err error) {
	reason, err := validateOwnerRoleDisposition(ctx, s, input)
	if err != nil {
		return result, err
	}
	now := s.clock.Now().UTC()
	keyHash := hashKey("platform-owner-identity-approve:v1", input.IdempotencyKey)
	requestHash := hashRequest("platform-owner-identity-approve:v1", struct {
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
			stored, storedErr := tx.GetIdentityStateChange(txCtx, input.ChangeID)
			if storedErr != nil {
				return storedErr
			}
			result = OwnerIdentityDispositionResult{ChangeID: input.ChangeID, TargetUserID: stored.TargetUserID, Action: stored.Action, State: stored.State, Replay: true}
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
		pending, lockErr := tx.LockPendingOwnerIdentityStateChange(txCtx, input.ChangeID, now)
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
		if !containsID(ownerIDs, pending.TargetUserID) {
			return ErrConcurrentChange
		}
		effective, lockErr := tx.CountEffectiveOwnerIdentities(txCtx, ownerIDs)
		if lockErr != nil {
			return lockErr
		}
		if effective < 2 {
			return ErrInvitationConflict
		}
		makerOwner, makerErr := requireCurrentIdentityStateMaker(txCtx, tx, pending.MakerID, pending.Action, ownerIDs)
		if makerErr != nil {
			return makerErr
		}
		checkerOwner := containsID(ownerIDs, checker.UserID)
		if !makerOwner && !checkerOwner {
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
		if pending.Action == DisableAdministrator {
			if identity.Disabled {
				return ErrConcurrentChange
			}
			targetEffective, countErr := tx.CountEffectiveOwnerIdentities(txCtx, []ID{pending.TargetUserID})
			if countErr != nil {
				return countErr
			}
			if targetEffective != 1 || effective <= 1 {
				return ErrInvitationConflict
			}
			lockErr = tx.SetAdministratorDisabled(txCtx, identity, now)
		} else {
			if !identity.Disabled {
				return ErrConcurrentChange
			}
			lockErr = tx.SetAdministratorActive(txCtx, identity, now)
		}
		if lockErr != nil {
			return lockErr
		}
		if lockErr = tx.AdvanceAdministratorProfile(txCtx, profile, now); lockErr != nil {
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
		ids, lockErr := newRoleChangeIDs(s.ids, 3)
		if lockErr != nil {
			return lockErr
		}
		if lockErr = tx.ClaimCommand(txCtx, ids[0], checker.UserID, "ADMINISTRATOR_CHANGE_APPROVE", keyHash, requestHash, pending.ChangeID, 200, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.AppendIdentityStateEvidence(txCtx, IdentityStateEvidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: input.CorrelationID, ActorID: checker.UserID, TargetUserID: pending.TargetUserID, ChangeID: pending.ChangeID, Action: pending.Action, State: "EXECUTED", Version: pending.Version + 1, OccurredAt: now}); lockErr != nil {
			return lockErr
		}
		result = OwnerIdentityDispositionResult{ChangeID: pending.ChangeID, TargetUserID: pending.TargetUserID, Action: pending.Action, State: "EXECUTED", RevokedSessions: revoked}
		notification = securityDeliveryRequest(pending.ChangeID, checker.UserID, identity, input.CorrelationID, now)
		return nil
	})
	if err == nil && notification != nil && s.delivery != nil {
		_ = s.delivery.DeliverPlatformIdentityStateChange(ctx, *notification, result.Action == DisableAdministrator)
	}
	return result, err
}

func (s *RoleChangeService) RejectOwnerIdentityStateChange(ctx context.Context, input OwnerIdentityDispositionInput) (result OwnerIdentityDispositionResult, err error) {
	reason, err := validateOwnerRoleDisposition(ctx, s, input)
	if err != nil {
		return result, err
	}
	now := s.clock.Now().UTC()
	keyHash := hashKey("platform-owner-identity-reject:v1", input.IdempotencyKey)
	requestHash := hashRequest("platform-owner-identity-reject:v1", struct {
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
			stored, storedErr := tx.GetIdentityStateChange(txCtx, input.ChangeID)
			if storedErr != nil {
				return storedErr
			}
			result = OwnerIdentityDispositionResult{ChangeID: input.ChangeID, TargetUserID: stored.TargetUserID, Action: stored.Action, State: stored.State, Replay: true}
			return nil
		}
		checker, _, authErr := requireRoleChangeChecker(txCtx, tx, input, now)
		if authErr != nil {
			return authErr
		}
		if _, lockErr := tx.LockGovernance(txCtx); lockErr != nil {
			return lockErr
		}
		pending, lockErr := tx.LockPendingOwnerIdentityStateChange(txCtx, input.ChangeID, now)
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
		if !containsID(ownerIDs, pending.TargetUserID) {
			return ErrConcurrentChange
		}
		effective, lockErr := tx.CountEffectiveOwnerIdentities(txCtx, ownerIDs)
		if lockErr != nil {
			return lockErr
		}
		if effective < 2 {
			return ErrInvitationConflict
		}
		makerOwner, makerErr := requireCurrentIdentityStateMaker(txCtx, tx, pending.MakerID, pending.Action, ownerIDs)
		if makerErr != nil {
			return makerErr
		}
		if !makerOwner && !containsID(ownerIDs, checker.UserID) {
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
		if lockErr = tx.AppendIdentityStateEvidence(txCtx, IdentityStateEvidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: input.CorrelationID, ActorID: checker.UserID, TargetUserID: pending.TargetUserID, ChangeID: pending.ChangeID, Action: pending.Action, State: "REJECTED", Version: pending.Version + 1, OccurredAt: now}); lockErr != nil {
			return lockErr
		}
		result = OwnerIdentityDispositionResult{ChangeID: pending.ChangeID, TargetUserID: pending.TargetUserID, Action: pending.Action, State: "REJECTED"}
		return nil
	})
	return result, err
}

func requireCurrentIdentityStateMaker(ctx context.Context, tx RoleChangeTx, makerID ID, action IdentityStateAction, ownerIDs []ID) (bool, error) {
	identity, err := tx.LockAdministratorIdentity(ctx, makerID)
	if err != nil {
		return false, err
	}
	if identity.Disabled {
		return false, ErrPlatformAccessForbidden
	}
	effective, err := tx.CountEffectiveOwnerIdentities(ctx, []ID{makerID})
	if err != nil {
		return false, err
	}
	if effective != 1 {
		return false, ErrPlatformAccessForbidden
	}
	platform, err := tx.GetPlatformAuthority(ctx, makerID)
	if err != nil {
		return false, ErrPlatformAccessForbidden
	}
	roles := make([]domain.Role, 0, len(platform.RoleKeys))
	for _, key := range platform.RoleKeys {
		role, parseErr := domain.ParseRole(key)
		if parseErr != nil {
			return false, ErrPlatformAccessForbidden
		}
		roles = append(roles, role)
	}
	facts := AuthorityFacts{Roles: roles}
	owner := containsID(ownerIDs, makerID) && containsRole(roles, domain.PlatformOwner)
	eligible := owner && facts.Grants("platform.admin.identity.reactivate")
	if action == DisableAdministrator {
		eligible = facts.Grants("platform.admin.identity.disable") && (owner || containsRole(roles, domain.SecurityAuditAdministrator))
	}
	if !eligible {
		return false, ErrPlatformAccessForbidden
	}
	return owner, nil
}
