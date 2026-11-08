package application

import (
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"errors"
	"strings"
	"time"

	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/platformadmin/domain"
)

type RoleChangeAction string

const (
	AssignRole RoleChangeAction = "ASSIGN_ROLE"
	RevokeRole RoleChangeAction = "REVOKE_ROLE"
)

type AdministratorProfile struct {
	UserID                     ID
	ContextGeneration, Version int64
}
type AdministratorIdentity struct {
	UserID       ID
	EmailDisplay string
	AuthEpoch    int64
	Disabled     bool
}
type RoleAssignment struct {
	ID, UserID ID
	Role       domain.Role
	Version    int64
}
type PendingOwnerRoleChange struct {
	ChangeID, TargetUserID, AssignmentID, MakerID                                      ID
	Action                                                                             RoleChangeAction
	ExpectedAdministratorVersion, ExpectedContextGeneration, ExpectedAssignmentVersion int64
	Reason                                                                             string
	ExpiresAt                                                                          time.Time
	Version                                                                            int64
}

type RoleChangeEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TargetUserID, ChangeID ID
	Action                                                            RoleChangeAction
	Role                                                              domain.Role
	State                                                             string
	Version                                                           int64
	OccurredAt                                                        time.Time
}

type RoleChangeTx interface {
	AuthorityTx
	GetCommand(context.Context, ID, string, [32]byte) (CommandRecord, bool, error)
	GetIdentityStateChange(context.Context, ID) (IdentityStateChangeResult, error)
	LockAdministratorProfile(context.Context, ID) (AdministratorProfile, error)
	LockAdministratorIdentity(context.Context, ID) (AdministratorIdentity, error)
	ActiveRoleExists(context.Context, ID, domain.Role) (bool, error)
	LockRoleAssignment(context.Context, ID, ID) (RoleAssignment, error)
	LockGovernance(context.Context) (GovernanceState, error)
	LockActiveOwnerIDs(context.Context) ([]ID, error)
	CountEffectiveOwnerIdentities(context.Context, []ID) (int64, error)
	LockPendingOwnerRoleChange(context.Context, ID, time.Time) (PendingOwnerRoleChange, error)
	LockPendingOwnerIdentityStateChange(context.Context, ID, time.Time) (PendingOwnerIdentityStateChange, error)
	DecideOwnerRoleChange(context.Context, ID, int64, ID, string, string, time.Time) error
	AdvanceOwnerGeneration(context.Context, int64, time.Time) error
	InsertRoleChange(context.Context, ID, RoleChangeInput, *ID, string, time.Time, *time.Time) error
	InsertRoleAssignment(context.Context, ID, ID, domain.Role, ID, string, time.Time) error
	RevokeRoleAssignment(context.Context, RoleAssignment, ID, string, time.Time) error
	AdvanceAdministratorProfile(context.Context, AdministratorProfile, time.Time) error
	AdvanceIdentityEpoch(context.Context, AdministratorIdentity, time.Time) error
	SetAdministratorDisabled(context.Context, AdministratorIdentity, time.Time) error
	SetAdministratorActive(context.Context, AdministratorIdentity, time.Time) error
	InsertIdentityStateChange(context.Context, ID, IdentityStateChangeInput, string, time.Time, *time.Time) error
	RevokeSessions(context.Context, ID, time.Time) (int64, error)
	ClaimCommand(context.Context, ID, ID, string, [32]byte, [32]byte, ID, int32, time.Time) error
	AppendRoleChangeEvidence(context.Context, RoleChangeEvidence) error
	AppendIdentityStateEvidence(context.Context, IdentityStateEvidence) error
	AppendAdministratorSessionRevocationEvidence(context.Context, AdministratorSessionRevocationEvidence) error
}
type RoleChangeUnitOfWork interface {
	WithinRoleChange(context.Context, func(context.Context, RoleChangeTx) error) error
}

type RoleChangeInput struct {
	ActorID, SessionID, TargetUserID, AssignmentID, CorrelationID, IdempotencyKey                                       ID
	PlatformContextGeneration, ExpectedAdministratorVersion, ExpectedTargetContextGeneration, ExpectedAssignmentVersion int64
	Action                                                                                                              RoleChangeAction
	Role                                                                                                                domain.Role
	Reason                                                                                                              string
}
type RoleChangeResult struct {
	ChangeID, TargetUserID, AssignmentID ID
	Action                               RoleChangeAction
	Role                                 domain.Role
	State                                string
	Version                              int64
	RevokedSessions                      int64
	Replay                               bool
}

type PlatformSecurityDeliveryRequest struct {
	SourceID, ActorID, RecipientUserID, CorrelationID ID
	Recipient                                         email.Address
	OccurredAt                                        time.Time
}
type PlatformSecurityDelivery interface {
	DeliverPlatformSessionRevocation(context.Context, PlatformSecurityDeliveryRequest) error
	DeliverPlatformIdentityStateChange(context.Context, PlatformSecurityDeliveryRequest, bool) error
	DeliverPlatformRoleChange(context.Context, PlatformSecurityDeliveryRequest, domain.Role, bool) error
}

type RoleChangeService struct {
	uow      RoleChangeUnitOfWork
	ids      InvitationIDGenerator
	clock    InvitationClock
	delivery PlatformSecurityDelivery
}

func NewRoleChangeService(uow RoleChangeUnitOfWork, ids InvitationIDGenerator, clock InvitationClock) (*RoleChangeService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("platform_role_change_service_invalid")
	}
	return &RoleChangeService{uow: uow, ids: ids, clock: clock}, nil
}
func (s *RoleChangeService) EnableSecurityDelivery(delivery PlatformSecurityDelivery) error {
	if s == nil || delivery == nil {
		return errors.New("platform_security_delivery_invalid")
	}
	s.delivery = delivery
	return nil
}

func (s *RoleChangeService) Change(ctx context.Context, input RoleChangeInput) (result RoleChangeResult, err error) {
	reason, err := validateRoleChangeInput(ctx, s, input)
	if err != nil {
		return result, err
	}
	now := s.clock.Now().UTC()
	purpose := "platform-role-change:" + string(input.Action) + ":v1"
	keyHash := hashKey(purpose, input.IdempotencyKey)
	requestHash := hashRequest(purpose, struct {
		Target, Assignment                                         ID
		Role                                                       string
		AdministratorVersion, ContextGeneration, AssignmentVersion int64
		Reason                                                     string
	}{input.TargetUserID, input.AssignmentID, string(input.Role), input.ExpectedAdministratorVersion, input.ExpectedTargetContextGeneration, input.ExpectedAssignmentVersion, reason})
	var notification *PlatformSecurityDeliveryRequest
	err = s.uow.WithinRoleChange(ctx, func(txCtx context.Context, tx RoleChangeTx) error {
		replayed, replayErr := roleChangeReplay(txCtx, tx, input.ActorID, keyHash, requestHash)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			stored, readErr := tx.GetIdentityStateChange(txCtx, replayed.TargetID)
			if readErr != nil {
				return readErr
			}
			state := stored.State
			result = RoleChangeResult{ChangeID: replayed.TargetID, TargetUserID: input.TargetUserID, AssignmentID: input.AssignmentID, Action: input.Action, Role: input.Role, State: state, Version: 1, Replay: true}
			return nil
		}
		capability := "platform.admin.role.assign"
		if input.Action == RevokeRole {
			capability = "platform.admin.role.revoke"
		}
		if _, authErr := requireRoleChangeOwner(txCtx, tx, input, capability, now); authErr != nil {
			return authErr
		}
		if input.ActorID == input.TargetUserID && (input.Action != RevokeRole || input.Role != domain.PlatformOwner) {
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
		if input.Action == AssignRole && identity.Disabled {
			return ErrPlatformAccessForbidden
		}
		var assignment RoleAssignment
		if input.Action == AssignRole {
			exists, existsErr := tx.ActiveRoleExists(txCtx, input.TargetUserID, input.Role)
			if existsErr != nil {
				return existsErr
			}
			if exists {
				return ErrInvitationConflict
			}
		} else {
			assignment, txErr = tx.LockRoleAssignment(txCtx, input.AssignmentID, input.TargetUserID)
			if txErr != nil {
				return txErr
			}
			if assignment.Role != input.Role || assignment.Version != input.ExpectedAssignmentVersion {
				return ErrConcurrentChange
			}
		}
		ids, txErr := newRoleChangeIDs(s.ids, 5)
		if txErr != nil {
			return txErr
		}
		changeID := ids[0]
		var ownerGovernance *GovernanceState
		if input.Role == domain.PlatformOwner {
			governance, ownerErr := tx.LockGovernance(txCtx)
			if ownerErr != nil {
				return ownerErr
			}
			ownerIDs, ownerErr := tx.LockActiveOwnerIDs(txCtx)
			if ownerErr != nil {
				return ownerErr
			}
			effective, ownerErr := tx.CountEffectiveOwnerIdentities(txCtx, ownerIDs)
			if ownerErr != nil {
				return ownerErr
			}
			if effective < 1 {
				return ErrInvitationConflict
			}
			if input.Action == RevokeRole {
				targetEffective, countErr := tx.CountEffectiveOwnerIdentities(txCtx, []ID{input.TargetUserID})
				if countErr != nil {
					return countErr
				}
				if targetEffective == 1 && effective <= 1 {
					return ErrInvitationConflict
				}
			}
			ownerGovernance = &governance
		}
		assignedID := input.AssignmentID
		if input.Action == AssignRole {
			assignedID = ids[4]
			if txErr = tx.InsertRoleAssignment(txCtx, assignedID, input.TargetUserID, input.Role, input.ActorID, reason, now); txErr != nil {
				return txErr
			}
		} else if txErr = tx.RevokeRoleAssignment(txCtx, assignment, input.ActorID, reason, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AdvanceAdministratorProfile(txCtx, profile, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AdvanceIdentityEpoch(txCtx, identity, now); txErr != nil {
			return txErr
		}
		revoked, txErr := tx.RevokeSessions(txCtx, identity.UserID, now)
		if txErr != nil {
			return txErr
		}
		if ownerGovernance != nil {
			if txErr = tx.AdvanceOwnerGeneration(txCtx, ownerGovernance.Version, now); txErr != nil {
				return txErr
			}
		}
		assignmentPointer := (*ID)(nil)
		if input.Action == RevokeRole {
			assignmentPointer = &input.AssignmentID
		}
		if txErr = tx.InsertRoleChange(txCtx, changeID, input, assignmentPointer, "EXECUTED", now, nil); txErr != nil {
			return txErr
		}
		if txErr = tx.ClaimCommand(txCtx, ids[1], input.ActorID, "ADMINISTRATOR_CHANGE_CREATE", keyHash, requestHash, changeID, 201, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AppendRoleChangeEvidence(txCtx, RoleChangeEvidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: input.CorrelationID, ActorID: input.ActorID, TargetUserID: input.TargetUserID, ChangeID: changeID, Action: input.Action, Role: input.Role, State: "EXECUTED", Version: 1, OccurredAt: now}); txErr != nil {
			return txErr
		}
		result = RoleChangeResult{ChangeID: changeID, TargetUserID: input.TargetUserID, AssignmentID: assignedID, Action: input.Action, Role: input.Role, State: "EXECUTED", Version: 1, RevokedSessions: revoked}
		notification = securityDeliveryRequest(changeID, input.ActorID, identity, input.CorrelationID, now)
		return nil
	})
	if err == nil && notification != nil && s.delivery != nil {
		_ = s.delivery.DeliverPlatformRoleChange(ctx, *notification, input.Role, input.Action == AssignRole)
	}
	return result, err
}

func securityDeliveryRequest(sourceID, actorID ID, identity AdministratorIdentity, correlationID ID, occurredAt time.Time) *PlatformSecurityDeliveryRequest {
	recipient, err := email.Parse(identity.EmailDisplay)
	if err != nil {
		return nil
	}
	return &PlatformSecurityDeliveryRequest{SourceID: sourceID, ActorID: actorID, RecipientUserID: identity.UserID, CorrelationID: correlationID, Recipient: recipient, OccurredAt: occurredAt}
}

func validateRoleChangeInput(ctx context.Context, service *RoleChangeService, input RoleChangeInput) (string, error) {
	reason := strings.TrimSpace(input.Reason)
	if ctx == nil || service == nil || service.uow == nil || service.ids == nil || service.clock == nil || input.ActorID.IsZero() || input.SessionID.IsZero() || input.TargetUserID.IsZero() || input.CorrelationID.IsZero() || input.IdempotencyKey.IsZero() || input.PlatformContextGeneration < 1 || input.ExpectedAdministratorVersion < 1 || input.ExpectedTargetContextGeneration < 1 || reason != input.Reason || len(reason) < 1 || len(reason) > 2000 {
		return "", ErrInvitationInvalid
	}
	if input.Action != AssignRole && input.Action != RevokeRole {
		return "", ErrInvitationInvalid
	}
	if _, err := domain.Capabilities(input.Role); err != nil {
		return "", ErrInvitationInvalid
	}
	if input.Action == AssignRole && (!input.AssignmentID.IsZero() || input.ExpectedAssignmentVersion != 0) {
		return "", ErrInvitationInvalid
	}
	if input.Action == RevokeRole && (input.AssignmentID.IsZero() || input.ExpectedAssignmentVersion < 1) {
		return "", ErrInvitationInvalid
	}
	return reason, nil
}
func requireRoleChangeOwner(ctx context.Context, tx RoleChangeTx, input RoleChangeInput, capability string, now time.Time) (AuthorityFacts, error) {
	facts, err := resolveAuthority(ctx, tx, AuthorityQuery{UserID: input.ActorID, SessionID: input.SessionID, ObservedAt: now})
	if err != nil {
		return AuthorityFacts{}, ErrPlatformAccessForbidden
	}
	if facts.ContextGeneration != input.PlatformContextGeneration {
		return AuthorityFacts{}, ErrPlatformContextStale
	}
	if !containsRole(facts.Roles, domain.PlatformOwner) || !facts.Grants(capability) {
		return AuthorityFacts{}, ErrPlatformAccessForbidden
	}
	if !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.After(now) || facts.AssuredAt.Before(now.Add(-recentPlatformAssurance)) {
		return AuthorityFacts{}, ErrPlatformAssuranceRequired
	}
	return facts, nil
}
func roleChangeReplay(ctx context.Context, tx RoleChangeTx, actor ID, key, request [32]byte) (*CommandRecord, error) {
	record, found, err := tx.GetCommand(ctx, actor, "ADMINISTRATOR_CHANGE_CREATE", key)
	if err != nil || !found {
		return nil, err
	}
	if record.RequestHash != request || record.TargetID.IsZero() {
		return nil, ErrInvitationConflict
	}
	return &record, nil
}
func newRoleChangeIDs(generator InvitationIDGenerator, count int) ([]ID, error) {
	ids := make([]ID, count)
	for i := range ids {
		id, err := generator.NewID()
		if err != nil || id.IsZero() {
			return nil, errors.New("platform_role_change_id_unavailable")
		}
		ids[i] = id
	}
	return ids, nil
}
