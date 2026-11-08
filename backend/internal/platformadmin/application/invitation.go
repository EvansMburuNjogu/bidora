package application

import (
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/platformadmin/domain"
	platforminvite "bidora/backend/internal/platformadmin/invitation"
)

const recentPlatformAssurance = 15 * time.Minute

var (
	ErrInvitationInvalid         = errors.New("platform_invitation_invalid")
	ErrInvitationConflict        = errors.New("platform_invitation_conflict")
	ErrInvitationNotFound        = errors.New("platform_invitation_not_found")
	ErrPlatformAccessForbidden   = errors.New("platform_access_forbidden")
	ErrPlatformAssuranceRequired = errors.New("platform_assurance_required")
	ErrPlatformContextStale      = errors.New("platform_context_stale")
)

type InvitationTokenGenerator interface {
	Generate() (platforminvite.Token, error)
}
type SecureInvitationTokenGenerator struct{}

func (SecureInvitationTokenGenerator) Generate() (platforminvite.Token, error) {
	return platforminvite.Generate()
}

type InvitationIDGenerator interface{ NewID() (ID, error) }
type InvitationClock interface{ Now() time.Time }

type PlatformInvitationDelivery interface {
	DeliverPlatformInvitation(context.Context, email.Address, platforminvite.Token, domain.Role, time.Time) error
}
type PlatformInvitationDeliveryRequest struct {
	InvitationID, ActorID, CorrelationID ID
	Recipient                            email.Address
	Token                                platforminvite.Token
	Role                                 domain.Role
	ExpiresAt                            time.Time
}
type EvidencedPlatformInvitationDelivery interface {
	DeliverPlatformInvitationWithEvidence(context.Context, PlatformInvitationDeliveryRequest) error
}

type InvitationAuthorizationPath string

const (
	InvitationOwnerDirect         InvitationAuthorizationPath = "OWNER_DIRECT"
	InvitationFirstOwnerException InvitationAuthorizationPath = "FIRST_OWNER_EXCEPTION"
	InvitationMakerChecker        InvitationAuthorizationPath = "MAKER_CHECKER"
)

type InvitationState string

const (
	InvitationPendingApproval InvitationState = "PENDING_APPROVAL"
	InvitationActive          InvitationState = "ACTIVE"
	InvitationAccepted        InvitationState = "ACCEPTED"
	InvitationCancelled       InvitationState = "CANCELLED"
)

type DeliveryState string

const (
	DeliveryNotReady    DeliveryState = "NOT_READY"
	DeliveryAccepted    DeliveryState = "ACCEPTED_FOR_DELIVERY"
	DeliveryUnconfirmed DeliveryState = "DELIVERY_UNCONFIRMED"
)

type InvitationRecord struct {
	ID                        ID
	EmailDisplay, EmailLookup string
	Role                      domain.Role
	AuthorizationPath         InvitationAuthorizationPath
	State                     InvitationState
	InvitedBy                 ID
	Reason                    string
	CreatedAt                 time.Time
	ActivatedAt, ExpiresAt    *time.Time
	Version                   int64
}

func (InvitationRecord) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform invitation]")
}

type CommandRecord struct {
	RequestHash    [32]byte
	TargetID       ID
	ResponseStatus int32
}

type PendingOwnerInvitation struct {
	ChangeID, MakerID ID
	ChangeVersion     int64
	ChangeExpiresAt   time.Time
	Invitation        InvitationRecord
}

type InvitationEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, InvitationID ID
	Action                                                  string
	Role                                                    domain.Role
	Version                                                 int64
	OccurredAt                                              time.Time
}

type InvitationTx interface {
	AuthorityTx
	GetCommand(context.Context, ID, string, [32]byte) (CommandRecord, bool, error)
	LockGovernance(context.Context) (GovernanceState, error)
	LockActiveOwnerIDs(context.Context) ([]ID, error)
	CountEffectiveOwnerIdentities(context.Context, []ID) (int64, error)
	FindOpenOwnerInvitation(context.Context) (InvitationRecord, bool, error)
	ExpireReplacedInvitationChanges(context.Context, string, time.Time) error
	ReplaceOpenInvitations(context.Context, string, time.Time) error
	InsertInvitation(context.Context, InvitationRecord, *[32]byte) error
	InsertOwnerInvitationChange(context.Context, ID, InvitationRecord, time.Time) error
	ConsumeFirstOwnerException(context.Context, int64, time.Time) error
	GetInvitation(context.Context, ID) (InvitationRecord, error)
	GetInvitationChangeID(context.Context, ID) (ID, bool, error)
	LockPendingOwnerInvitation(context.Context, ID, time.Time) (PendingOwnerInvitation, error)
	ActivateOwnerInvitation(context.Context, ID, int64, [32]byte, time.Time, time.Time) error
	RejectOwnerInvitation(context.Context, ID, int64, time.Time) error
	DecideOwnerInvitationChange(context.Context, ID, int64, ID, string, string, time.Time) error
	CancelInvitation(context.Context, ID, int64, time.Time) (InvitationRecord, error)
	ExpireInvitationChange(context.Context, ID, time.Time) error
	LockInvitationByToken(context.Context, [32]byte, time.Time) (InvitationRecord, error)
	LockInvitee(context.Context, ID, ID, string, time.Time) (BootstrapIdentity, error)
	HasTenantTies(context.Context, ID, string, time.Time) (bool, error)
	PromoteIdentity(context.Context, ID, int64, time.Time) (int64, error)
	RevokeSessions(context.Context, ID, time.Time) (int64, error)
	CreateAdministratorProfile(context.Context, ID, time.Time) error
	AssignInvitedRole(context.Context, ID, ID, domain.Role, ID, string, time.Time) error
	AcceptInvitation(context.Context, ID, int64, ID, time.Time) error
	AdvanceOwnerGeneration(context.Context, int64, time.Time) error
	ClaimCommand(context.Context, ID, ID, string, [32]byte, [32]byte, ID, int32, time.Time) error
	AppendInvitationEvidence(context.Context, InvitationEvidence) error
}

type InvitationUnitOfWork interface {
	WithinInvitation(context.Context, func(context.Context, InvitationTx) error) error
}

type IssueInvitationInput struct {
	ActorID, SessionID, CorrelationID, IdempotencyKey ID
	PlatformContextGeneration                         int64
	Email, Reason                                     string
	Role                                              domain.Role
}
type IssueInvitationResult struct {
	InvitationID       ID
	State              InvitationState
	Role               domain.Role
	Version            int64
	ExpiresAt          *time.Time
	GovernanceChangeID *ID
	Delivery           DeliveryState
	Replay             bool
}

func (IssueInvitationInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform invitation issue input]")
}
func (IssueInvitationResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform invitation issue result]")
}

type CancelInvitationInput struct {
	ActorID, SessionID, InvitationID, CorrelationID, IdempotencyKey ID
	PlatformContextGeneration, ExpectedVersion                      int64
}

type AcceptInvitationInput struct {
	ActorID, SessionID, CorrelationID, IdempotencyKey ID
	PresentedToken                                    string
}
type OwnerInvitationDispositionInput struct {
	CheckerID, SessionID, ChangeID, CorrelationID, IdempotencyKey ID
	PlatformContextGeneration, ExpectedChangeVersion              int64
	Reason                                                        string
}
type OwnerInvitationDispositionResult struct {
	ChangeID, InvitationID ID
	State                  string
	ExpiresAt              *time.Time
	Delivery               DeliveryState
	Replay                 bool
}

type AcceptInvitationResult struct {
	InvitationID    ID
	Role            domain.Role
	RevokedSessions int64
	Replay          bool
}

func (AcceptInvitationInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform invitation acceptance]")
}

type InvitationService struct {
	uow      InvitationUnitOfWork
	tokens   InvitationTokenGenerator
	ids      InvitationIDGenerator
	clock    InvitationClock
	delivery PlatformInvitationDelivery
}

func NewInvitationService(uow InvitationUnitOfWork, tokens InvitationTokenGenerator, ids InvitationIDGenerator, clock InvitationClock, delivery PlatformInvitationDelivery) (*InvitationService, error) {
	if uow == nil || tokens == nil || ids == nil || clock == nil || delivery == nil {
		return nil, errors.New("platform_invitation_service_invalid")
	}
	return &InvitationService{uow: uow, tokens: tokens, ids: ids, clock: clock, delivery: delivery}, nil
}

func (s *InvitationService) Issue(ctx context.Context, input IssueInvitationInput) (result IssueInvitationResult, err error) {
	address, reason, err := validateIssueInput(ctx, input)
	if err != nil || s == nil || s.uow == nil || s.tokens == nil || s.ids == nil || s.clock == nil || s.delivery == nil {
		return result, ErrInvitationInvalid
	}
	now := s.clock.Now().UTC()
	requestHash := hashRequest("platform-invitation-issue:v1", struct{ Email, Role, Reason string }{address.Lookup(), string(input.Role), reason})
	keyHash := hashKey("platform-invitation-issue:v1", input.IdempotencyKey)
	var deliveryToken platforminvite.Token
	var recipient email.Address
	err = s.uow.WithinInvitation(ctx, func(txCtx context.Context, tx InvitationTx) error {
		replayed, replayErr := invitationReplay(txCtx, tx, input.ActorID, "INVITATION_ISSUE", keyHash, requestHash)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			record, readErr := tx.GetInvitation(txCtx, replayed.TargetID)
			if readErr != nil {
				return readErr
			}
			delivery := DeliveryUnconfirmed
			if record.State == InvitationPendingApproval {
				delivery = DeliveryNotReady
			}
			var changeID *ID
			if record.State == InvitationPendingApproval {
				id, found, changeErr := tx.GetInvitationChangeID(txCtx, record.ID)
				if changeErr != nil || !found {
					if changeErr != nil {
						return changeErr
					}
					return ErrInvitationConflict
				}
				changeID = &id
			}
			result = issueResult(record, changeID, delivery, true)
			return nil
		}
		if _, authErr := requireInvitationOwner(txCtx, tx, input.ActorID, input.SessionID, input.PlatformContextGeneration, now); authErr != nil {
			return authErr
		}
		_, txErr := tx.LockGovernance(txCtx)
		if txErr != nil {
			return txErr
		}
		ownerIDs, txErr := tx.LockActiveOwnerIDs(txCtx)
		if txErr != nil {
			return txErr
		}
		effectiveOwners, txErr := tx.CountEffectiveOwnerIdentities(txCtx, ownerIDs)
		if txErr != nil {
			return txErr
		}
		if effectiveOwners < 1 {
			return ErrPlatformAccessForbidden
		}

		state := InvitationActive
		authorizationPath := InvitationOwnerDirect
		var tokenHash *[32]byte
		var activatedAt, expiresAt *time.Time
		var changeID *ID
		deliveryToken, txErr = s.tokens.Generate()
		if txErr != nil || deliveryToken.IsZero() {
			return errors.New("platform_invitation_token_unavailable")
		}
		hash := deliveryToken.Hash()
		tokenHash = &hash
		activated := now
		expires, expiryErr := platforminvite.ExpiresAt(now)
		if expiryErr != nil {
			return expiryErr
		}
		activatedAt, expiresAt = &activated, &expires
		recipient = address
		ids, txErr := s.newIDs(5)
		if txErr != nil {
			return txErr
		}
		record := InvitationRecord{ID: ids[0], EmailDisplay: address.Display(), EmailLookup: address.Lookup(), Role: input.Role, AuthorizationPath: authorizationPath, State: state, InvitedBy: input.ActorID, Reason: reason, CreatedAt: now, ActivatedAt: activatedAt, ExpiresAt: expiresAt, Version: 1}
		if txErr = tx.ExpireReplacedInvitationChanges(txCtx, address.Lookup(), now); txErr != nil {
			return txErr
		}
		if txErr = tx.ReplaceOpenInvitations(txCtx, address.Lookup(), now); txErr != nil {
			return txErr
		}
		if txErr = tx.InsertInvitation(txCtx, record, tokenHash); txErr != nil {
			return txErr
		}
		if txErr = tx.ClaimCommand(txCtx, ids[2], input.ActorID, "INVITATION_ISSUE", keyHash, requestHash, record.ID, 201, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AppendInvitationEvidence(txCtx, InvitationEvidence{AuditID: ids[3], OutboxID: ids[4], CorrelationID: input.CorrelationID, ActorID: input.ActorID, InvitationID: record.ID, Action: "ISSUED", Role: input.Role, Version: 1, OccurredAt: now}); txErr != nil {
			return txErr
		}
		result = issueResult(record, changeID, DeliveryNotReady, false)
		return nil
	})
	if err != nil {
		return IssueInvitationResult{}, err
	}
	if !deliveryToken.IsZero() {
		result.Delivery = DeliveryAccepted
		var deliverErr error
		if delivery, ok := s.delivery.(EvidencedPlatformInvitationDelivery); ok {
			deliverErr = delivery.DeliverPlatformInvitationWithEvidence(ctx, PlatformInvitationDeliveryRequest{InvitationID: result.InvitationID, ActorID: input.ActorID, CorrelationID: input.CorrelationID, Recipient: recipient, Token: deliveryToken, Role: result.Role, ExpiresAt: *result.ExpiresAt})
		} else {
			deliverErr = s.delivery.DeliverPlatformInvitation(ctx, recipient, deliveryToken, result.Role, *result.ExpiresAt)
		}
		if deliverErr != nil {
			result.Delivery = DeliveryUnconfirmed
		}
	}
	return result, nil
}

func validateIssueInput(ctx context.Context, input IssueInvitationInput) (email.Address, string, error) {
	if ctx == nil || input.ActorID.IsZero() || input.SessionID.IsZero() || input.CorrelationID.IsZero() || input.IdempotencyKey.IsZero() || input.PlatformContextGeneration < 1 {
		return email.Address{}, "", ErrInvitationInvalid
	}
	address, err := email.Parse(input.Email)
	reason := strings.TrimSpace(input.Reason)
	if err != nil || reason != input.Reason || len(reason) < 1 || len(reason) > 2000 {
		return email.Address{}, "", ErrInvitationInvalid
	}
	if _, err = domain.Capabilities(input.Role); err != nil {
		return email.Address{}, "", ErrInvitationInvalid
	}
	return address, reason, nil
}

func (s *InvitationService) Cancel(ctx context.Context, input CancelInvitationInput) error {
	if ctx == nil || s == nil || s.uow == nil || input.ActorID.IsZero() || input.SessionID.IsZero() || input.InvitationID.IsZero() || input.CorrelationID.IsZero() || input.IdempotencyKey.IsZero() || input.PlatformContextGeneration < 1 || input.ExpectedVersion < 1 {
		return ErrInvitationInvalid
	}
	now := s.clock.Now().UTC()
	requestHash := hashRequest("platform-invitation-cancel:v1", struct {
		Invitation ID
		Version    int64
	}{input.InvitationID, input.ExpectedVersion})
	keyHash := hashKey("platform-invitation-cancel:v1", input.IdempotencyKey)
	return s.uow.WithinInvitation(ctx, func(txCtx context.Context, tx InvitationTx) error {
		replayed, err := invitationReplay(txCtx, tx, input.ActorID, "INVITATION_CANCEL", keyHash, requestHash)
		if err != nil || replayed != nil {
			return err
		}
		if _, err = requireInvitationOwner(txCtx, tx, input.ActorID, input.SessionID, input.PlatformContextGeneration, now); err != nil {
			return err
		}
		record, err := tx.CancelInvitation(txCtx, input.InvitationID, input.ExpectedVersion, now)
		if err != nil {
			return err
		}
		if err = tx.ExpireInvitationChange(txCtx, record.ID, now); err != nil {
			return err
		}
		ids, err := s.newIDs(3)
		if err != nil {
			return err
		}
		if err = tx.ClaimCommand(txCtx, ids[0], input.ActorID, "INVITATION_CANCEL", keyHash, requestHash, record.ID, 204, now); err != nil {
			return err
		}
		return tx.AppendInvitationEvidence(txCtx, InvitationEvidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: input.CorrelationID, ActorID: input.ActorID, InvitationID: record.ID, Action: "CANCELLED", Role: record.Role, Version: record.Version, OccurredAt: now})
	})
}

func (s *InvitationService) Accept(ctx context.Context, input AcceptInvitationInput) (result AcceptInvitationResult, err error) {
	if ctx == nil || s == nil || s.uow == nil || input.ActorID.IsZero() || input.SessionID.IsZero() || input.CorrelationID.IsZero() || input.IdempotencyKey.IsZero() {
		return result, ErrInvitationInvalid
	}
	tokenHash, err := platforminvite.HashPresented(input.PresentedToken)
	if err != nil {
		return result, ErrInvitationInvalid
	}
	now := s.clock.Now().UTC()
	requestHash := hashRequest("platform-invitation-accept:v1", struct{ TokenHash [32]byte }{tokenHash})
	keyHash := hashKey("platform-invitation-accept:v1", input.IdempotencyKey)
	err = s.uow.WithinInvitation(ctx, func(txCtx context.Context, tx InvitationTx) error {
		replayed, replayErr := invitationReplay(txCtx, tx, input.ActorID, "INVITATION_ACCEPT", keyHash, requestHash)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			result = AcceptInvitationResult{InvitationID: replayed.TargetID, Replay: true}
			return nil
		}
		record, txErr := tx.LockInvitationByToken(txCtx, tokenHash, now)
		if txErr != nil {
			return ErrInvitationInvalid
		}
		identity, txErr := tx.LockInvitee(txCtx, input.ActorID, input.SessionID, record.EmailLookup, now)
		if txErr != nil {
			return ErrInvitationInvalid
		}
		tied, txErr := tx.HasTenantTies(txCtx, identity.UserID, identity.AffiliationKey, now)
		if txErr != nil {
			return txErr
		}
		if tied {
			return ErrInvitationInvalid
		}
		ids, txErr := s.newIDs(4)
		if txErr != nil {
			return txErr
		}
		newEpoch, txErr := tx.PromoteIdentity(txCtx, identity.UserID, identity.AuthEpoch, now)
		if txErr != nil {
			return txErr
		}
		if newEpoch != identity.AuthEpoch+1 {
			return ErrConcurrentChange
		}
		revoked, txErr := tx.RevokeSessions(txCtx, identity.UserID, now)
		if txErr != nil {
			return txErr
		}
		if txErr = tx.CreateAdministratorProfile(txCtx, identity.UserID, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AssignInvitedRole(txCtx, ids[0], identity.UserID, record.Role, record.InvitedBy, record.Reason, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AcceptInvitation(txCtx, record.ID, record.Version, identity.UserID, now); txErr != nil {
			return txErr
		}
		if record.Role == domain.PlatformOwner {
			governance, lockErr := tx.LockGovernance(txCtx)
			if lockErr != nil {
				return lockErr
			}
			if txErr = tx.AdvanceOwnerGeneration(txCtx, governance.Version, now); txErr != nil {
				return txErr
			}
		}
		if txErr = tx.ClaimCommand(txCtx, ids[1], identity.UserID, "INVITATION_ACCEPT", keyHash, requestHash, record.ID, 204, now); txErr != nil {
			return txErr
		}
		if txErr = tx.AppendInvitationEvidence(txCtx, InvitationEvidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: input.CorrelationID, ActorID: identity.UserID, InvitationID: record.ID, Action: "ACCEPTED", Role: record.Role, Version: record.Version + 1, OccurredAt: now}); txErr != nil {
			return txErr
		}
		result = AcceptInvitationResult{InvitationID: record.ID, Role: record.Role, RevokedSessions: revoked}
		return nil
	})
	return result, err
}

func requireInvitationOwner(ctx context.Context, tx InvitationTx, actorID, sessionID ID, generation int64, now time.Time) (AuthorityFacts, error) {
	facts, err := resolveAuthority(ctx, tx, AuthorityQuery{UserID: actorID, SessionID: sessionID, ObservedAt: now})
	if err != nil {
		return AuthorityFacts{}, ErrPlatformAccessForbidden
	}
	if facts.ContextGeneration != generation {
		return AuthorityFacts{}, ErrPlatformContextStale
	}
	if !facts.Grants("platform.admin.invite") || !containsRole(facts.Roles, domain.PlatformOwner) {
		return AuthorityFacts{}, ErrPlatformAccessForbidden
	}
	if !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.After(now) || facts.AssuredAt.Before(now.Add(-recentPlatformAssurance)) {
		return AuthorityFacts{}, ErrPlatformAssuranceRequired
	}
	return facts, nil
}
func containsRole(roles []domain.Role, want domain.Role) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}
func invitationReplay(ctx context.Context, tx InvitationTx, actor ID, operation string, key, request [32]byte) (*CommandRecord, error) {
	record, found, err := tx.GetCommand(ctx, actor, operation, key)
	if err != nil || !found {
		return nil, err
	}
	if record.RequestHash != request || record.TargetID.IsZero() {
		return nil, ErrInvitationConflict
	}
	return &record, nil
}
func issueResult(record InvitationRecord, change *ID, delivery DeliveryState, replay bool) IssueInvitationResult {
	return IssueInvitationResult{InvitationID: record.ID, State: record.State, Role: record.Role, Version: record.Version, ExpiresAt: record.ExpiresAt, GovernanceChangeID: change, Delivery: delivery, Replay: replay}
}
func (s *InvitationService) newIDs(count int) ([]ID, error) {
	ids := make([]ID, count)
	for index := range ids {
		id, err := s.ids.NewID()
		if err != nil || id.IsZero() {
			return nil, errors.New("platform_invitation_id_unavailable")
		}
		ids[index] = id
	}
	return ids, nil
}
func hashKey(purpose string, id ID) [32]byte {
	return sha256.Sum256(append(append([]byte(purpose), 0), id[:]...))
}
func hashRequest(purpose string, value any) [32]byte {
	body, _ := json.Marshal(value)
	return sha256.Sum256(append(append([]byte(purpose), 0), body...))
}
