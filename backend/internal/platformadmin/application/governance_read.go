package application

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/platformadmin/domain"
)

type ReadAuthority struct {
	ActorID, SessionID        ID
	PlatformContextGeneration int64
	ObservedAt                time.Time
}
type PageCursor struct {
	BeforeTime *time.Time
	BeforeID   *ID
	Limit      int32
}
type AdministratorFilter struct {
	State       *string
	Role        *domain.Role
	Email       *string
	EmailLookup *string // Resolved from the approved mailbox policy, never an HTTP selector.
}
type AdministratorProfileView struct {
	UserID                     ID
	ContextGeneration, Version int64
	CreatedAt, UpdatedAt       time.Time
}
type AdministratorIdentityView struct {
	UserID                 ID
	Email                  string
	Disabled, TOTPEnrolled bool
}
type AssignmentView struct {
	ID, UserID            ID
	Role                  domain.Role
	State, Source, Reason string
	AssignedBy            *ID
	AssignedAt            time.Time
	RevokedBy             *ID
	RevokedAt             *time.Time
	RevocationReason      *string
	Version               int64
}
type AdministratorView struct {
	Profile        AdministratorProfileView
	Identity       AdministratorIdentityView
	Assignments    []AssignmentView
	EffectiveOwner bool
}
type InvitationView struct {
	ID                 ID
	Email              string
	Role               domain.Role
	State              string
	Version            int64
	CreatedAt          time.Time
	ExpiresAt          *time.Time
	Delivery           DeliveryState
	GovernanceChangeID *ID
}
type ChangeView struct {
	ID                                                                                 ID
	Action                                                                             string
	SessionsRevoked                                                                    bool
	TargetUserID, InvitationID, AssignmentID                                           *ID
	Role                                                                               *domain.Role
	ExpectedAdministratorVersion, ExpectedContextGeneration, ExpectedAssignmentVersion *int64
	Reason                                                                             string
	DecisionReason                                                                     *string
	State                                                                              string
	Version                                                                            int64
	MakerID                                                                            ID
	CheckerID                                                                          *ID
	CreatedAt                                                                          time.Time
	ExpiresAt, DecidedAt                                                               *time.Time
}
type RoleCatalogueView struct {
	Role         domain.Role
	Capabilities []string
}
type AdministratorList struct {
	Administrators             []AdministratorView
	EffectiveOwnerCount        int64
	ContinuityWarning, HasMore bool
	NextCursor                 *PageCursor
}
type InvitationList struct {
	Invitations []InvitationView
	HasMore     bool
	NextCursor  *PageCursor
}
type AssignmentHistory struct {
	UserID      ID
	Assignments []AssignmentView
	HasMore     bool
	NextCursor  *PageCursor
}
type ChangeList struct {
	Changes    []ChangeView
	HasMore    bool
	NextCursor *PageCursor
}

type GovernanceReadTx interface {
	AuthorityTx
	CountEffectiveOwners(context.Context) (int64, error)
	ListAdministratorProfiles(context.Context, AdministratorFilter, PageCursor) ([]AdministratorProfileView, error)
	GetAdministratorProfile(context.Context, ID) (AdministratorProfileView, error)
	ListAdministratorIdentities(context.Context, []ID) ([]AdministratorIdentityView, error)
	ListLatestAssignments(context.Context, []ID) ([]AssignmentView, error)
	ListAssignmentHistory(context.Context, ID, PageCursor) ([]AssignmentView, error)
	ListInvitations(context.Context, PageCursor) ([]InvitationView, error)
	GetInvitation(context.Context, ID) (InvitationView, error)
	ListChanges(context.Context, *string, PageCursor, time.Time) ([]ChangeView, error)
	GetChange(context.Context, ID) (ChangeView, error)
}
type GovernanceReadUnitOfWork interface {
	WithinGovernanceRead(context.Context, func(context.Context, GovernanceReadTx) error) error
}
type GovernanceReadService struct{ uow GovernanceReadUnitOfWork }

func NewGovernanceReadService(uow GovernanceReadUnitOfWork) (*GovernanceReadService, error) {
	if uow == nil {
		return nil, errors.New("platform_governance_read_service_invalid")
	}
	return &GovernanceReadService{uow: uow}, nil
}

func (s *GovernanceReadService) Roles(ctx context.Context, auth ReadAuthority) (result []RoleCatalogueView, err error) {
	err = s.withRead(ctx, auth, false, func(context.Context, GovernanceReadTx) error {
		for _, role := range domain.Roles() {
			capabilities, capErr := domain.Capabilities(role)
			if capErr != nil {
				return capErr
			}
			result = append(result, RoleCatalogueView{Role: role, Capabilities: capabilities})
		}
		return nil
	})
	return result, err
}
func (s *GovernanceReadService) Administrators(ctx context.Context, auth ReadAuthority, filter AdministratorFilter, page PageCursor) (result AdministratorList, err error) {
	if err = validatePage(page); err != nil || filter.EmailLookup != nil {
		return result, ErrInvitationInvalid
	}
	if filter.State != nil && *filter.State != "ACTIVE" && *filter.State != "DISABLED" {
		return result, ErrInvitationInvalid
	}
	if filter.Role != nil {
		if _, roleErr := domain.Capabilities(*filter.Role); roleErr != nil {
			return result, ErrInvitationInvalid
		}
	}
	if filter.Email != nil {
		address, parseErr := email.Parse(*filter.Email)
		if parseErr != nil {
			return result, ErrInvitationInvalid
		}
		lookup := address.Lookup()
		filter.EmailLookup = &lookup
	}
	err = s.withRead(ctx, auth, false, func(txCtx context.Context, tx GovernanceReadTx) error {
		profiles, readErr := tx.ListAdministratorProfiles(txCtx, filter, page)
		if readErr != nil {
			return readErr
		}
		if len(profiles) > int(page.Limit) {
			result.HasMore = true
			profiles = profiles[:page.Limit]
			last := profiles[len(profiles)-1]
			result.NextCursor = nextCursor(last.CreatedAt, last.UserID, page.Limit)
		}
		ids := make([]ID, len(profiles))
		for i, p := range profiles {
			ids[i] = p.UserID
		}
		identities, readErr := tx.ListAdministratorIdentities(txCtx, ids)
		if readErr != nil {
			return readErr
		}
		assignments, readErr := tx.ListLatestAssignments(txCtx, ids)
		if readErr != nil {
			return readErr
		}
		result.Administrators, readErr = composeAdministrators(profiles, identities, assignments)
		if readErr != nil {
			return readErr
		}
		if filter.State != nil {
			filtered := result.Administrators[:0]
			for _, administrator := range result.Administrators {
				state := "ACTIVE"
				if administrator.Identity.Disabled {
					state = "DISABLED"
				}
				if state == *filter.State {
					filtered = append(filtered, administrator)
				}
			}
			result.Administrators = filtered
		}
		result.EffectiveOwnerCount, readErr = tx.CountEffectiveOwners(txCtx)
		if readErr != nil {
			return readErr
		}
		result.ContinuityWarning = result.EffectiveOwnerCount < 2
		return nil
	})
	return result, err
}
func (s *GovernanceReadService) Administrator(ctx context.Context, auth ReadAuthority, userID ID) (result AdministratorView, err error) {
	if userID.IsZero() {
		return result, ErrAuthorityNotFound
	}
	err = s.withRead(ctx, auth, false, func(txCtx context.Context, tx GovernanceReadTx) error {
		profile, readErr := tx.GetAdministratorProfile(txCtx, userID)
		if readErr != nil {
			return readErr
		}
		identities, readErr := tx.ListAdministratorIdentities(txCtx, []ID{userID})
		if readErr != nil {
			return readErr
		}
		assignments, readErr := tx.ListLatestAssignments(txCtx, []ID{userID})
		if readErr != nil {
			return readErr
		}
		items, readErr := composeAdministrators([]AdministratorProfileView{profile}, identities, assignments)
		if readErr != nil {
			return readErr
		}
		result = items[0]
		return nil
	})
	return result, err
}
func (s *GovernanceReadService) Invitation(ctx context.Context, auth ReadAuthority, id ID) (result InvitationView, err error) {
	if id.IsZero() {
		return result, ErrAuthorityNotFound
	}
	err = s.withRead(ctx, auth, false, func(txCtx context.Context, tx GovernanceReadTx) error {
		result, err = tx.GetInvitation(txCtx, id)
		return err
	})
	return result, err
}
func (s *GovernanceReadService) Invitations(ctx context.Context, auth ReadAuthority, page PageCursor) (result InvitationList, err error) {
	if err = validatePage(page); err != nil {
		return result, err
	}
	err = s.withRead(ctx, auth, false, func(txCtx context.Context, tx GovernanceReadTx) error {
		result.Invitations, err = tx.ListInvitations(txCtx, page)
		if err == nil && len(result.Invitations) > int(page.Limit) {
			result.HasMore = true
			result.Invitations = result.Invitations[:page.Limit]
			last := result.Invitations[len(result.Invitations)-1]
			result.NextCursor = nextCursor(last.CreatedAt, last.ID, page.Limit)
		}
		return err
	})
	return result, err
}
func (s *GovernanceReadService) AssignmentHistory(ctx context.Context, auth ReadAuthority, userID ID, page PageCursor) (result AssignmentHistory, err error) {
	if userID.IsZero() {
		return result, ErrAuthorityNotFound
	}
	if err = validatePage(page); err != nil {
		return result, err
	}
	err = s.withRead(ctx, auth, false, func(txCtx context.Context, tx GovernanceReadTx) error {
		if _, readErr := tx.GetAdministratorProfile(txCtx, userID); readErr != nil {
			return readErr
		}
		result.UserID = userID
		result.Assignments, err = tx.ListAssignmentHistory(txCtx, userID, page)
		if err == nil && len(result.Assignments) > int(page.Limit) {
			result.HasMore = true
			result.Assignments = result.Assignments[:page.Limit]
			last := result.Assignments[len(result.Assignments)-1]
			result.NextCursor = nextCursor(last.AssignedAt, last.ID, page.Limit)
		}
		return err
	})
	return result, err
}
func (s *GovernanceReadService) Changes(ctx context.Context, auth ReadAuthority, state *string, page PageCursor) (result ChangeList, err error) {
	if err = validatePage(page); err != nil {
		return result, err
	}
	if state != nil && *state != "PENDING_APPROVAL" && *state != "EXECUTED" && *state != "REJECTED" && *state != "EXPIRED" {
		return result, ErrInvitationInvalid
	}
	err = s.withRead(ctx, auth, true, func(txCtx context.Context, tx GovernanceReadTx) error {
		result.Changes, err = tx.ListChanges(txCtx, state, page, auth.ObservedAt.UTC())
		if err != nil {
			return err
		}
		for index := range result.Changes {
			normalizeChangeState(&result.Changes[index], auth.ObservedAt)
		}
		if len(result.Changes) > int(page.Limit) {
			result.HasMore = true
			result.Changes = result.Changes[:page.Limit]
			last := result.Changes[len(result.Changes)-1]
			result.NextCursor = nextCursor(last.CreatedAt, last.ID, page.Limit)
		}
		return nil
	})
	return result, err
}
func (s *GovernanceReadService) Change(ctx context.Context, auth ReadAuthority, changeID ID) (result ChangeView, err error) {
	if changeID.IsZero() {
		return result, ErrAuthorityNotFound
	}
	err = s.withRead(ctx, auth, false, func(txCtx context.Context, tx GovernanceReadTx) error {
		result, err = tx.GetChange(txCtx, changeID)
		if err == nil {
			normalizeChangeState(&result, auth.ObservedAt)
		}
		return err
	})
	return result, err
}

func normalizeChangeState(change *ChangeView, observedAt time.Time) {
	if change != nil && change.State == "PENDING_APPROVAL" && change.ExpiresAt != nil && !change.ExpiresAt.After(observedAt) {
		change.State = "EXPIRED"
	}
}

func (s *GovernanceReadService) withRead(ctx context.Context, auth ReadAuthority, governance bool, fn func(context.Context, GovernanceReadTx) error) error {
	if ctx == nil || s == nil || s.uow == nil || auth.ActorID.IsZero() || auth.SessionID.IsZero() || auth.PlatformContextGeneration < 1 || auth.ObservedAt.IsZero() {
		return ErrPlatformAccessForbidden
	}
	return s.uow.WithinGovernanceRead(ctx, func(txCtx context.Context, tx GovernanceReadTx) error {
		facts, resolveErr := resolveAuthority(txCtx, tx, AuthorityQuery{UserID: auth.ActorID, SessionID: auth.SessionID, ObservedAt: auth.ObservedAt.UTC()})
		if resolveErr != nil {
			return ErrPlatformAccessForbidden
		}
		if facts.ContextGeneration != auth.PlatformContextGeneration {
			return ErrPlatformContextStale
		}
		if !facts.Grants("platform.admin.read") {
			return ErrPlatformAccessForbidden
		}
		if facts.AssuranceLevel != "AAL2" || facts.AssuranceMethod != "TOTP" || facts.AssuredAt.After(auth.ObservedAt) || facts.AssuredAt.Before(auth.ObservedAt.Add(-recentPlatformAssurance)) {
			return ErrPlatformAssuranceRequired
		}
		if governance && !facts.Grants("platform.governance.approve") && !facts.Grants("platform.governance.review") {
			return ErrPlatformAccessForbidden
		}
		return fn(txCtx, tx)
	})
}
func nextCursor(at time.Time, id ID, limit int32) *PageCursor {
	at = at.UTC()
	return &PageCursor{BeforeTime: &at, BeforeID: &id, Limit: limit}
}
func validatePage(page PageCursor) error {
	if page.Limit < 1 || page.Limit > 100 || (page.BeforeTime == nil) != (page.BeforeID == nil) {
		return ErrInvitationInvalid
	}
	if page.BeforeTime != nil && (page.BeforeTime.IsZero() || page.BeforeID.IsZero()) {
		return ErrInvitationInvalid
	}
	return nil
}
func composeAdministrators(profiles []AdministratorProfileView, identities []AdministratorIdentityView, assignments []AssignmentView) ([]AdministratorView, error) {
	identityByID := map[ID]AdministratorIdentityView{}
	for _, identity := range identities {
		identityByID[identity.UserID] = identity
	}
	assignmentsByID := map[ID][]AssignmentView{}
	for _, assignment := range assignments {
		assignmentsByID[assignment.UserID] = append(assignmentsByID[assignment.UserID], assignment)
	}
	result := make([]AdministratorView, len(profiles))
	for i, profile := range profiles {
		identity, ok := identityByID[profile.UserID]
		if !ok {
			return nil, ErrAuthorityNotFound
		}
		item := AdministratorView{Profile: profile, Identity: identity, Assignments: assignmentsByID[profile.UserID]}
		for _, assignment := range item.Assignments {
			if assignment.Role == domain.PlatformOwner && assignment.State == "ACTIVE" && !identity.Disabled && identity.TOTPEnrolled {
				item.EffectiveOwner = true
			}
		}
		result[i] = item
	}
	return result, nil
}
