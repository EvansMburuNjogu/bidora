package application

import (
	"bidora/backend/internal/identity/securitypolicy"
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"errors"
	"strings"
	"time"

	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/platformadmin/domain"
	platforminvite "bidora/backend/internal/platformadmin/invitation"
)

func (s *InvitationService) ApproveOwnerInvitation(ctx context.Context, input OwnerInvitationDispositionInput) (result OwnerInvitationDispositionResult, err error) {
	reason, err := validateDispositionInput(ctx, s, input)
	if err != nil {
		return result, err
	}
	now := s.clock.Now().UTC()
	keyHash := hashKey("platform-owner-invitation-approve:v1", input.IdempotencyKey)
	requestHash := hashRequest("platform-owner-invitation-approve:v1", struct {
		ChangeID ID
		Version  int64
		Reason   string
	}{input.ChangeID, input.ExpectedChangeVersion, reason})
	var token platforminvite.Token
	var recipient email.Address
	var role domain.Role
	err = s.uow.WithinInvitation(ctx, func(txCtx context.Context, tx InvitationTx) error {
		replayed, replayErr := invitationReplay(txCtx, tx, input.CheckerID, "ADMINISTRATOR_CHANGE_APPROVE", keyHash, requestHash)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			result = OwnerInvitationDispositionResult{ChangeID: input.ChangeID, InvitationID: replayed.TargetID, State: "EXECUTED", Delivery: DeliveryUnconfirmed, Replay: true}
			return nil
		}
		checker, identity, authErr := requireInvitationChecker(txCtx, tx, input, now)
		if authErr != nil {
			return authErr
		}
		pending, lockErr := tx.LockPendingOwnerInvitation(txCtx, input.ChangeID, now)
		if lockErr != nil {
			return lockErr
		}
		if pending.ChangeVersion != input.ExpectedChangeVersion {
			return ErrInvitationConflict
		}
		if pending.MakerID == checker.UserID || identity.AffiliationKey == pending.Invitation.EmailLookup {
			return ErrPlatformAccessForbidden
		}
		if makerErr := requireCurrentEffectiveMaker(txCtx, tx, pending.MakerID); makerErr != nil {
			return makerErr
		}
		token, lockErr = s.tokens.Generate()
		if lockErr != nil || token.IsZero() {
			return errors.New("platform_invitation_token_unavailable")
		}
		expiresAt, expiryErr := platforminvite.ExpiresAt(now)
		if expiryErr != nil {
			return expiryErr
		}
		if lockErr = tx.ActivateOwnerInvitation(txCtx, pending.Invitation.ID, pending.Invitation.Version, token.Hash(), now, expiresAt); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.DecideOwnerInvitationChange(txCtx, pending.ChangeID, pending.ChangeVersion, checker.UserID, "EXECUTED", reason, now); lockErr != nil {
			return lockErr
		}
		ids, idErr := s.newIDs(3)
		if idErr != nil {
			return idErr
		}
		if lockErr = tx.ClaimCommand(txCtx, ids[0], checker.UserID, "ADMINISTRATOR_CHANGE_APPROVE", keyHash, requestHash, pending.Invitation.ID, 200, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.AppendInvitationEvidence(txCtx, InvitationEvidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: input.CorrelationID, ActorID: checker.UserID, InvitationID: pending.Invitation.ID, Action: "ACTIVATED", Role: pending.Invitation.Role, Version: pending.Invitation.Version + 1, OccurredAt: now}); lockErr != nil {
			return lockErr
		}
		recipient, lockErr = email.Parse(pending.Invitation.EmailDisplay)
		if lockErr != nil {
			return errors.New("platform_invitation_recipient_invalid")
		}
		role = pending.Invitation.Role
		result = OwnerInvitationDispositionResult{ChangeID: pending.ChangeID, InvitationID: pending.Invitation.ID, State: "EXECUTED", ExpiresAt: &expiresAt, Delivery: DeliveryAccepted}
		return nil
	})
	if err != nil {
		return OwnerInvitationDispositionResult{}, err
	}
	if !token.IsZero() {
		var deliveryErr error
		if delivery, ok := s.delivery.(EvidencedPlatformInvitationDelivery); ok {
			deliveryErr = delivery.DeliverPlatformInvitationWithEvidence(ctx, PlatformInvitationDeliveryRequest{InvitationID: result.InvitationID, ActorID: input.CheckerID, CorrelationID: input.CorrelationID, Recipient: recipient, Token: token, Role: role, ExpiresAt: *result.ExpiresAt})
		} else {
			deliveryErr = s.delivery.DeliverPlatformInvitation(ctx, recipient, token, role, *result.ExpiresAt)
		}
		if deliveryErr != nil {
			result.Delivery = DeliveryUnconfirmed
		}
	}
	return result, nil
}

func (s *InvitationService) RejectOwnerInvitation(ctx context.Context, input OwnerInvitationDispositionInput) (result OwnerInvitationDispositionResult, err error) {
	reason, err := validateDispositionInput(ctx, s, input)
	if err != nil {
		return result, err
	}
	now := s.clock.Now().UTC()
	keyHash := hashKey("platform-owner-invitation-reject:v1", input.IdempotencyKey)
	requestHash := hashRequest("platform-owner-invitation-reject:v1", struct {
		ChangeID ID
		Version  int64
		Reason   string
	}{input.ChangeID, input.ExpectedChangeVersion, reason})
	err = s.uow.WithinInvitation(ctx, func(txCtx context.Context, tx InvitationTx) error {
		replayed, replayErr := invitationReplay(txCtx, tx, input.CheckerID, "ADMINISTRATOR_CHANGE_REJECT", keyHash, requestHash)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			result = OwnerInvitationDispositionResult{ChangeID: input.ChangeID, InvitationID: replayed.TargetID, State: "REJECTED", Delivery: DeliveryNotReady, Replay: true}
			return nil
		}
		checker, identity, authErr := requireInvitationChecker(txCtx, tx, input, now)
		if authErr != nil {
			return authErr
		}
		pending, lockErr := tx.LockPendingOwnerInvitation(txCtx, input.ChangeID, now)
		if lockErr != nil {
			return lockErr
		}
		if pending.ChangeVersion != input.ExpectedChangeVersion {
			return ErrInvitationConflict
		}
		if pending.MakerID == checker.UserID || identity.AffiliationKey == pending.Invitation.EmailLookup {
			return ErrPlatformAccessForbidden
		}
		if makerErr := requireCurrentEffectiveMaker(txCtx, tx, pending.MakerID); makerErr != nil {
			return makerErr
		}
		if lockErr = tx.RejectOwnerInvitation(txCtx, pending.Invitation.ID, pending.Invitation.Version, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.DecideOwnerInvitationChange(txCtx, pending.ChangeID, pending.ChangeVersion, checker.UserID, "REJECTED", reason, now); lockErr != nil {
			return lockErr
		}
		ids, idErr := s.newIDs(3)
		if idErr != nil {
			return idErr
		}
		if lockErr = tx.ClaimCommand(txCtx, ids[0], checker.UserID, "ADMINISTRATOR_CHANGE_REJECT", keyHash, requestHash, pending.Invitation.ID, 200, now); lockErr != nil {
			return lockErr
		}
		if lockErr = tx.AppendInvitationEvidence(txCtx, InvitationEvidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: input.CorrelationID, ActorID: checker.UserID, InvitationID: pending.Invitation.ID, Action: "REJECTED", Role: pending.Invitation.Role, Version: pending.Invitation.Version + 1, OccurredAt: now}); lockErr != nil {
			return lockErr
		}
		result = OwnerInvitationDispositionResult{ChangeID: pending.ChangeID, InvitationID: pending.Invitation.ID, State: "REJECTED", Delivery: DeliveryNotReady}
		return nil
	})
	return result, err
}

func validateDispositionInput(ctx context.Context, service *InvitationService, input OwnerInvitationDispositionInput) (string, error) {
	reason := strings.TrimSpace(input.Reason)
	if ctx == nil || service == nil || service.uow == nil || service.tokens == nil || service.ids == nil || service.clock == nil || service.delivery == nil || input.CheckerID.IsZero() || input.SessionID.IsZero() || input.ChangeID.IsZero() || input.CorrelationID.IsZero() || input.IdempotencyKey.IsZero() || input.PlatformContextGeneration < 1 || input.ExpectedChangeVersion < 1 || reason != input.Reason || len(reason) < 1 || len(reason) > 2000 {
		return "", ErrInvitationInvalid
	}
	return reason, nil
}

func requireInvitationChecker(ctx context.Context, tx InvitationTx, input OwnerInvitationDispositionInput, now time.Time) (AuthorityFacts, IdentityAuthorityFacts, error) {
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
	if !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.After(now) || !securitypolicy.Recent(ctx, facts.AssuredAt, now) {
		return AuthorityFacts{}, IdentityAuthorityFacts{}, ErrPlatformAssuranceRequired
	}
	return facts, identity, nil
}

func requireCurrentEffectiveMaker(ctx context.Context, tx InvitationTx, makerID ID) error {
	ownerIDs, err := tx.LockActiveOwnerIDs(ctx)
	if err != nil {
		return err
	}
	if !containsID(ownerIDs, makerID) {
		return ErrPlatformAccessForbidden
	}
	total, err := tx.CountEffectiveOwnerIdentities(ctx, ownerIDs)
	if err != nil {
		return err
	}
	if total < 2 {
		return ErrInvitationConflict
	}
	count, err := tx.CountEffectiveOwnerIdentities(ctx, []ID{makerID})
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrPlatformAccessForbidden
	}
	return nil
}

func containsID(ids []ID, wanted ID) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}
