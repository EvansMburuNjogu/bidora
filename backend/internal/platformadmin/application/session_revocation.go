package application

import (
	"context"
	"strings"
	"time"

	"bidora/backend/internal/platformadmin/domain"
)

type AdministratorSessionRevocationInput struct {
	ActorID, SessionID, TargetUserID, CorrelationID, IdempotencyKey                          ID
	PlatformContextGeneration, ExpectedAdministratorVersion, ExpectedTargetContextGeneration int64
	Reason                                                                                   string
}
type AdministratorSessionRevocationResult struct {
	TargetUserID    ID
	RevokedSessions int64
	Replay          bool
}
type AdministratorSessionRevocationEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TargetUserID ID
	TargetAuthEpoch                                         int64
	Reason                                                  string
	OccurredAt                                              time.Time
}

func (s *RoleChangeService) RevokeAdministratorSessions(ctx context.Context, input AdministratorSessionRevocationInput) (result AdministratorSessionRevocationResult, err error) {
	reason, err := validateAdministratorSessionRevocation(ctx, s, input)
	if err != nil {
		return result, err
	}
	now := s.clock.Now().UTC()
	keyHash := hashKey("platform-administrator-sessions-revoke:v1", input.IdempotencyKey)
	requestHash := hashRequest("platform-administrator-sessions-revoke:v1", struct {
		Target                                  ID
		AdministratorVersion, ContextGeneration int64
		Reason                                  string
	}{input.TargetUserID, input.ExpectedAdministratorVersion, input.ExpectedTargetContextGeneration, reason})
	var notification *PlatformSecurityDeliveryRequest
	err = s.uow.WithinRoleChange(ctx, func(txCtx context.Context, tx RoleChangeTx) error {
		replay, found, replayErr := tx.GetCommand(txCtx, input.ActorID, "ADMINISTRATOR_SESSIONS_REVOKE", keyHash)
		if replayErr != nil {
			return replayErr
		}
		if found {
			if replay.RequestHash != requestHash || replay.TargetID != input.TargetUserID {
				return ErrInvitationConflict
			}
			result = AdministratorSessionRevocationResult{TargetUserID: input.TargetUserID, Replay: true}
			return nil
		}
		facts, authErr := resolveAuthority(txCtx, tx, AuthorityQuery{UserID: input.ActorID, SessionID: input.SessionID, ObservedAt: now})
		if authErr != nil {
			return ErrPlatformAccessForbidden
		}
		if facts.ContextGeneration != input.PlatformContextGeneration {
			return ErrPlatformContextStale
		}
		eligible := facts.Grants("platform.admin.session.revoke") && (containsRole(facts.Roles, domain.PlatformOwner) || containsRole(facts.Roles, domain.SecurityAuditAdministrator))
		if !eligible {
			return ErrPlatformAccessForbidden
		}
		if facts.AssuranceLevel != "AAL2" || facts.AssuranceMethod != "TOTP" || facts.AssuredAt.After(now) || facts.AssuredAt.Before(now.Add(-recentPlatformAssurance)) {
			return ErrPlatformAssuranceRequired
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
		if txErr = tx.AdvanceIdentityEpoch(txCtx, identity, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AdvanceAdministratorProfile(txCtx, profile, now); txErr != nil {
			return txErr
		}
		revoked, txErr := tx.RevokeSessions(txCtx, identity.UserID, now)
		if txErr != nil {
			return txErr
		}
		ids, txErr := newRoleChangeIDs(s.ids, 3)
		if txErr != nil {
			return txErr
		}
		if txErr = tx.ClaimCommand(txCtx, ids[0], input.ActorID, "ADMINISTRATOR_SESSIONS_REVOKE", keyHash, requestHash, input.TargetUserID, 204, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AppendAdministratorSessionRevocationEvidence(txCtx, AdministratorSessionRevocationEvidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: input.CorrelationID, ActorID: input.ActorID, TargetUserID: input.TargetUserID, TargetAuthEpoch: identity.AuthEpoch + 1, Reason: reason, OccurredAt: now}); txErr != nil {
			return txErr
		}
		result = AdministratorSessionRevocationResult{TargetUserID: input.TargetUserID, RevokedSessions: revoked}
		notification = securityDeliveryRequest(ids[0], input.ActorID, identity, input.CorrelationID, now)
		return nil
	})
	if err == nil && notification != nil && s.delivery != nil {
		_ = s.delivery.DeliverPlatformSessionRevocation(ctx, *notification)
	}
	return result, err
}
func validateAdministratorSessionRevocation(ctx context.Context, s *RoleChangeService, input AdministratorSessionRevocationInput) (string, error) {
	reason := strings.TrimSpace(input.Reason)
	if ctx == nil || s == nil || s.uow == nil || s.ids == nil || s.clock == nil || input.ActorID.IsZero() || input.SessionID.IsZero() || input.TargetUserID.IsZero() || input.CorrelationID.IsZero() || input.IdempotencyKey.IsZero() || input.PlatformContextGeneration < 1 || input.ExpectedAdministratorVersion < 1 || input.ExpectedTargetContextGeneration < 1 || reason != input.Reason || len(reason) < 1 || len(reason) > 2000 {
		return "", ErrInvitationInvalid
	}
	return reason, nil
}
