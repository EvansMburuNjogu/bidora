package application

import (
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"strings"
	"time"

	"bidora/backend/internal/platformadmin/domain"
)

type IdentityStateAction string

const (
	DisableAdministrator    IdentityStateAction = "DISABLE_IDENTITY"
	ReactivateAdministrator IdentityStateAction = "REACTIVATE_IDENTITY"
)

type IdentityStateChangeInput struct {
	ActorID, SessionID, TargetUserID, CorrelationID, IdempotencyKey                          ID
	PlatformContextGeneration, ExpectedAdministratorVersion, ExpectedTargetContextGeneration int64
	Action                                                                                   IdentityStateAction
	Reason                                                                                   string
}
type PendingOwnerIdentityStateChange struct {
	ChangeID, TargetUserID, MakerID                         ID
	Action                                                  IdentityStateAction
	ExpectedAdministratorVersion, ExpectedContextGeneration int64
	Reason                                                  string
	ExpiresAt                                               time.Time
	Version                                                 int64
}
type IdentityStateChangeResult struct {
	ChangeID, TargetUserID ID
	Action                 IdentityStateAction
	State                  string
	RevokedSessions        int64
	Replay                 bool
}
type IdentityStateEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TargetUserID, ChangeID ID
	Action                                                            IdentityStateAction
	State                                                             string
	Version                                                           int64
	OccurredAt                                                        time.Time
}

func (s *RoleChangeService) ChangeIdentityState(ctx context.Context, input IdentityStateChangeInput) (result IdentityStateChangeResult, err error) {
	reason, err := validateIdentityStateInput(ctx, s, input)
	if err != nil {
		return result, err
	}
	now := s.clock.Now().UTC()
	purpose := "platform-identity-state:" + string(input.Action) + ":v1"
	keyHash := hashKey(purpose, input.IdempotencyKey)
	requestHash := hashRequest(purpose, struct {
		Target                                  ID
		AdministratorVersion, ContextGeneration int64
		Reason                                  string
	}{input.TargetUserID, input.ExpectedAdministratorVersion, input.ExpectedTargetContextGeneration, reason})
	var notification *PlatformSecurityDeliveryRequest
	err = s.uow.WithinRoleChange(ctx, func(txCtx context.Context, tx RoleChangeTx) error {
		replay, found, replayErr := tx.GetCommand(txCtx, input.ActorID, "ADMINISTRATOR_CHANGE_CREATE", keyHash)
		if replayErr != nil {
			return replayErr
		}
		if found {
			if replay.RequestHash != requestHash || replay.TargetID.IsZero() {
				return ErrInvitationConflict
			}
			stored, storedErr := tx.GetIdentityStateChange(txCtx, replay.TargetID)
			if storedErr != nil {
				return storedErr
			}
			if stored.TargetUserID != input.TargetUserID || stored.Action != input.Action {
				return ErrInvitationConflict
			}
			stored.ChangeID, stored.Replay = replay.TargetID, true
			result = stored
			return nil
		}
		actor, authErr := requireIdentityStateActor(txCtx, tx, input, now)
		if authErr != nil {
			return authErr
		}
		if input.ActorID == input.TargetUserID {
			return ErrPlatformAccessForbidden
		}
		profile, txErr := tx.LockAdministratorProfile(txCtx, input.TargetUserID)
		if txErr != nil {
			return txErr
		}
		if profile.Version != input.ExpectedAdministratorVersion || profile.ContextGeneration != input.ExpectedTargetContextGeneration {
			return ErrConcurrentChange
		}
		identity, txErr := tx.LockAdministratorIdentity(txCtx, input.TargetUserID)
		if txErr != nil {
			return txErr
		}
		if input.Action == DisableAdministrator && identity.Disabled {
			return ErrConcurrentChange
		}
		if input.Action == ReactivateAdministrator && !identity.Disabled {
			return ErrConcurrentChange
		}
		governance, txErr := tx.LockGovernance(txCtx)
		if txErr != nil {
			return txErr
		}
		ownerIDs, txErr := tx.LockActiveOwnerIDs(txCtx)
		if txErr != nil {
			return txErr
		}
		targetOwner := containsID(ownerIDs, input.TargetUserID)
		ids, txErr := newRoleChangeIDs(s.ids, 4)
		if txErr != nil {
			return txErr
		}
		changeID := ids[0]
		if targetOwner {
			if !containsRole(actor.Roles, domain.PlatformOwner) {
				return ErrPlatformAccessForbidden
			}
			effective, ownerErr := tx.CountEffectiveOwnerIdentities(txCtx, ownerIDs)
			if ownerErr != nil {
				return ownerErr
			}
			if input.Action == DisableAdministrator {
				targetEffective, countErr := tx.CountEffectiveOwnerIdentities(txCtx, []ID{input.TargetUserID})
				if countErr != nil {
					return countErr
				}
				if targetEffective == 1 && effective <= 1 {
					return ErrInvitationConflict
				}
			}
		}
		if input.Action == DisableAdministrator {
			txErr = tx.SetAdministratorDisabled(txCtx, identity, now)
		} else {
			txErr = tx.SetAdministratorActive(txCtx, identity, now)
		}
		if txErr != nil {
			return txErr
		}
		if txErr = tx.AdvanceAdministratorProfile(txCtx, profile, now); txErr != nil {
			return txErr
		}
		revoked, txErr := tx.RevokeSessions(txCtx, identity.UserID, now)
		if txErr != nil {
			return txErr
		}
		if targetOwner {
			if txErr = tx.AdvanceOwnerGeneration(txCtx, governance.Version, now); txErr != nil {
				return txErr
			}
		}
		if txErr = tx.InsertIdentityStateChange(txCtx, changeID, input, "EXECUTED", now, nil); txErr != nil {
			return txErr
		}
		if txErr = tx.ClaimCommand(txCtx, ids[1], input.ActorID, "ADMINISTRATOR_CHANGE_CREATE", keyHash, requestHash, changeID, 201, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AppendIdentityStateEvidence(txCtx, IdentityStateEvidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: input.CorrelationID, ActorID: input.ActorID, TargetUserID: input.TargetUserID, ChangeID: changeID, Action: input.Action, State: "EXECUTED", Version: 1, OccurredAt: now}); txErr != nil {
			return txErr
		}
		result = IdentityStateChangeResult{ChangeID: changeID, TargetUserID: input.TargetUserID, Action: input.Action, State: "EXECUTED", RevokedSessions: revoked}
		notification = securityDeliveryRequest(changeID, input.ActorID, identity, input.CorrelationID, now)
		return nil
	})
	if err == nil && notification != nil && s.delivery != nil {
		_ = s.delivery.DeliverPlatformIdentityStateChange(ctx, *notification, input.Action == DisableAdministrator)
	}
	return result, err
}

func validateIdentityStateInput(ctx context.Context, s *RoleChangeService, input IdentityStateChangeInput) (string, error) {
	reason := strings.TrimSpace(input.Reason)
	if ctx == nil || s == nil || s.uow == nil || s.ids == nil || s.clock == nil || input.ActorID.IsZero() || input.SessionID.IsZero() || input.TargetUserID.IsZero() || input.CorrelationID.IsZero() || input.IdempotencyKey.IsZero() || input.PlatformContextGeneration < 1 || input.ExpectedAdministratorVersion < 1 || input.ExpectedTargetContextGeneration < 1 || reason != input.Reason || len(reason) < 1 || len(reason) > 2000 {
		return "", ErrInvitationInvalid
	}
	if input.Action != DisableAdministrator && input.Action != ReactivateAdministrator {
		return "", ErrInvitationInvalid
	}
	return reason, nil
}
func requireIdentityStateActor(ctx context.Context, tx RoleChangeTx, input IdentityStateChangeInput, now time.Time) (AuthorityFacts, error) {
	facts, err := resolveAuthority(ctx, tx, AuthorityQuery{UserID: input.ActorID, SessionID: input.SessionID, ObservedAt: now})
	if err != nil {
		return AuthorityFacts{}, ErrPlatformAccessForbidden
	}
	if facts.ContextGeneration != input.PlatformContextGeneration {
		return AuthorityFacts{}, ErrPlatformContextStale
	}
	eligible := false
	if input.Action == DisableAdministrator {
		eligible = facts.Grants("platform.admin.identity.disable") && (containsRole(facts.Roles, domain.PlatformOwner) || containsRole(facts.Roles, domain.SecurityAuditAdministrator))
	} else {
		eligible = facts.Grants("platform.admin.identity.reactivate") && containsRole(facts.Roles, domain.PlatformOwner)
	}
	if !eligible {
		return AuthorityFacts{}, ErrPlatformAccessForbidden
	}
	if !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.After(now) || facts.AssuredAt.Before(now.Add(-recentPlatformAssurance)) {
		return AuthorityFacts{}, ErrPlatformAssuranceRequired
	}
	return facts, nil
}
