package application

import (
	"context"
	"errors"
	"time"
)

var ErrOwnershipContinuityLimit = errors.New("ownership_continuity_limit")

type ContinuityMember struct {
	MembershipID, UserID ID
	ContextGeneration    int64
}
type OwnershipActivity struct {
	UserID                                                                   ID
	OwnedBidCount, OpenRequirementCount, OpenReviewCount, OverdueReviewCount int64
	LastActivityAt                                                           *time.Time
}
type MemberContinuity struct {
	MembershipID, UserID                                                     ID
	ContextGeneration                                                        int64
	OwnedBidCount, OpenRequirementCount, OpenReviewCount, OverdueReviewCount int64
	LastActivityAt                                                           *time.Time
}
type OwnershipContinuityReport struct {
	TenantID                                                                                                          ID
	AssignmentGeneration                                                                                              int64
	Members                                                                                                           []MemberContinuity
	OrphanedOwnership                                                                                                 []OwnershipActivity
	MemberCount                                                                                                       int
	CanRevokeWithoutReview, AutomaticTransferSupported, ExplicitOffboardingSupported, LastActiveAdminPolicyConfigured bool
	Blockers                                                                                                          []string
	ObservedAt                                                                                                        time.Time
}
type OwnershipContinuityAuthorizer interface {
	AuthorizeOwnershipContinuity(context.Context, WorkflowSubject) (WorkflowAuthority, []ContinuityMember, error)
}
type OwnershipContinuityStore interface {
	ListOwnershipContinuityActivity(context.Context, WorkflowAuthority, time.Time) ([]OwnershipActivity, error)
}
type OwnershipContinuityService struct {
	auth  OwnershipContinuityAuthorizer
	store OwnershipContinuityStore
	clock CaptureClock
}

func NewOwnershipContinuityService(auth OwnershipContinuityAuthorizer, store OwnershipContinuityStore, clock CaptureClock) (*OwnershipContinuityService, error) {
	if auth == nil || store == nil || clock == nil {
		return nil, errors.New("ownership_continuity_service_invalid")
	}
	return &OwnershipContinuityService{auth: auth, store: store, clock: clock}, nil
}
func (s *OwnershipContinuityService) Get(ctx context.Context, subject WorkflowSubject) (OwnershipContinuityReport, error) {
	if ctx == nil || !validWorkflowSubject(subject) {
		return OwnershipContinuityReport{}, safe("ownership_continuity_invalid", nil)
	}
	a, members, err := s.auth.AuthorizeOwnershipContinuity(ctx, subject)
	if err != nil {
		if err.Error() == "ownership_continuity_limit" {
			return OwnershipContinuityReport{}, safe("ownership_continuity_limit", err)
		}
		return OwnershipContinuityReport{}, safe("ownership_continuity_not_authorized", err)
	}
	observed := s.clock.Now().UTC()
	activities, err := s.store.ListOwnershipContinuityActivity(ctx, a, observed)
	if errors.Is(err, ErrOwnershipContinuityLimit) {
		return OwnershipContinuityReport{}, safe("ownership_continuity_limit", err)
	}
	if err != nil {
		return OwnershipContinuityReport{}, safe("ownership_continuity_unavailable", err)
	}
	byUser := make(map[ID]OwnershipActivity, len(activities))
	for _, row := range activities {
		byUser[row.UserID] = row
	}
	active := make(map[ID]struct{}, len(members))
	result := OwnershipContinuityReport{TenantID: a.TenantID, AssignmentGeneration: a.AssignmentGeneration, MemberCount: len(members), Members: make([]MemberContinuity, len(members)), ObservedAt: observed, ExplicitOffboardingSupported: true, LastActiveAdminPolicyConfigured: true, Blockers: []string{"DIRECT_REVOCATION_REQUIRES_OFFBOARDING_REVIEW", "AUTOMATIC_OWNERSHIP_TRANSFER_NOT_SUPPORTED"}}
	for i, m := range members {
		active[m.UserID] = struct{}{}
		row := byUser[m.UserID]
		result.Members[i] = MemberContinuity{MembershipID: m.MembershipID, UserID: m.UserID, ContextGeneration: m.ContextGeneration, OwnedBidCount: row.OwnedBidCount, OpenRequirementCount: row.OpenRequirementCount, OpenReviewCount: row.OpenReviewCount, OverdueReviewCount: row.OverdueReviewCount, LastActivityAt: row.LastActivityAt}
	}
	for _, row := range activities {
		if _, ok := active[row.UserID]; !ok && (row.OwnedBidCount > 0 || row.OpenRequirementCount > 0 || row.OpenReviewCount > 0) {
			result.OrphanedOwnership = append(result.OrphanedOwnership, row)
		}
	}
	if len(result.OrphanedOwnership) > 0 {
		result.Blockers = append(result.Blockers, "ORPHANED_WORK_OWNERSHIP")
	}
	result.CanRevokeWithoutReview = false
	result.AutomaticTransferSupported = false
	result.LastActiveAdminPolicyConfigured = true
	return result, nil
}
