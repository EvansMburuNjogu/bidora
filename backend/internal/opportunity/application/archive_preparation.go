package application

import (
	"context"
	"errors"
	"time"
)

var ErrArchivePreparationNotFound = errors.New("archive_preparation_not_found")

type BidArchivePreparation struct {
	BidID, ProposalVersionID                                                                                                                                                 ID
	WorkspaceState, CurrentReportedOutcome                                                                                                                                   string
	WorkspaceVersion                                                                                                                                                         int64
	ArchivedAt                                                                                                                                                               *time.Time
	RequirementCount, OpenRequirementCount, OpenReviewThreadCount, PendingUploadCount, PendingJobCount, PackageManifestCount, ExternalReportCount, ActiveExternalReportCount int64
	ArchivePolicyConfigured, ReadOnlyClosureSupported, HasSubmissionConfirmation, CanArchive                                                                                 bool
	Blockers                                                                                                                                                                 []string
	ObservedAt                                                                                                                                                               time.Time
}
type ArchivePreparationStore interface {
	GetBidArchivePreparation(context.Context, WorkflowAuthority, ID, time.Time) (BidArchivePreparation, error)
}
type ArchivePreparationService struct {
	auth  TechnicalResponseAuthorizer
	store ArchivePreparationStore
	clock CaptureClock
}

func NewArchivePreparationService(auth TechnicalResponseAuthorizer, store ArchivePreparationStore, clock CaptureClock) (*ArchivePreparationService, error) {
	if auth == nil || store == nil || clock == nil {
		return nil, errors.New("archive_preparation_service_invalid")
	}
	return &ArchivePreparationService{auth: auth, store: store, clock: clock}, nil
}
func (s *ArchivePreparationService) Get(ctx context.Context, subject WorkflowSubject, bidID ID) (BidArchivePreparation, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return BidArchivePreparation{}, safe("archive_preparation_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return BidArchivePreparation{}, safe("archive_preparation_not_authorized", err)
	}
	result, err := s.store.GetBidArchivePreparation(ctx, a, bidID, s.clock.Now().UTC())
	if errors.Is(err, ErrArchivePreparationNotFound) {
		return BidArchivePreparation{}, safe("archive_preparation_not_found", err)
	}
	if err != nil {
		return BidArchivePreparation{}, safe("archive_preparation_unavailable", err)
	}
	result.ArchivePolicyConfigured = true
	result.ReadOnlyClosureSupported = true
	result.CanArchive = true
	result.Blockers = nil
	if result.ArchivedAt != nil {
		result.Blockers = append(result.Blockers, "WORKSPACE_ALREADY_ARCHIVED")
		result.CanArchive = false
	}
	if result.OpenReviewThreadCount > 0 {
		result.Blockers = append(result.Blockers, "OPEN_REVIEW_THREADS")
		result.CanArchive = false
	}
	if result.PendingUploadCount > 0 {
		result.Blockers = append(result.Blockers, "PENDING_UPLOADS")
		result.CanArchive = false
	}
	if result.PendingJobCount > 0 {
		result.Blockers = append(result.Blockers, "PENDING_BACKGROUND_JOBS")
		result.CanArchive = false
	}
	eligible := result.WorkspaceState == "NO_BID" || (result.WorkspaceState == "SUBMITTED" && result.HasSubmissionConfirmation && terminalReportedOutcome(result.CurrentReportedOutcome))
	if !eligible {
		result.Blockers = append(result.Blockers, "WORKSPACE_STATE_OR_OUTCOME_INELIGIBLE")
		result.CanArchive = false
	}
	return result, nil
}
