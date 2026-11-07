package httpapi

import (
	"errors"

	"bidora/backend/internal/health"
	opportunitytransport "bidora/backend/internal/opportunity/transport"
	"github.com/gofiber/fiber/v3"
)

const LoginPath = "/v1/auth/login"
const RegistrationPath = "/v1/auth/register"
const EmailVerificationPath = "/v1/auth/verify-email"
const InvitationIssuePath = "/v1/shared/invitations"
const InvitationAcceptPath = "/v1/shared/invitations/accept"
const InvitationDirectoryPath = "/v1/shared/invitations"
const InvitationPath = "/v1/shared/invitations/:invitation_id"
const MembershipDirectoryPath = "/v1/me/memberships"
const MembershipDirectoryManagementPath = "/v1/shared/memberships"
const MembershipPath = "/v1/shared/memberships/:membership_id"
const LogoutPath = "/v1/auth/logout"
const SessionBootstrapPath = "/v1/auth/session/bootstrap"
const ManualCapturePath = "/v1/tenderpreneur/opportunities/manual"

type PilotRoutes struct {
	Browser                 *BrowserSessionMiddleware
	Login                   *LoginHandler
	PasswordChange          *PasswordChangeHandler
	Registration            *RegistrationHandler
	EmailVerification       *EmailVerificationHandler
	EmailVerificationResend *EmailVerificationResendHandler
	InvitationIssue         *InvitationIssueHandler
	InvitationAccept        *InvitationAcceptHandler
	InvitationDirectory     *InvitationDirectoryHandler
	InvitationCancel        *InvitationCancellationHandler
	Memberships             *MembershipDirectoryHandler
	MembershipRevoke        *MembershipRevocationHandler
	MembershipOffboarding   *MembershipOffboardingHandler
	Logout                  *LogoutHandler
	Bootstrap               *SessionBootstrapHandler
	SessionInventory        *SessionInventoryHandler
	SessionAdministration   *SessionAdministrationHandler
	AccountType             *AccountTypeHandler
	RecoveryPreparation     *RecoveryPreparationHandler
	PasswordRecovery        *PasswordRecoveryHandler
	MFAReadiness            *MFAReadinessHandler
	MFA                     *MFAHandler
	Capture                 *ManualCaptureHandler
	OpportunityWorkflow     *OpportunityWorkflowHandler
	OpportunityRead         *OpportunityReadHandler
	Requirements            *RequirementHandler
	DocumentUploads         *DocumentUploadHandler
	AIRequirementReviews    *AIRequirementReviewHandler
	RequirementEvidence     *RequirementEvidenceHandler
	TechnicalResponses      *TechnicalResponseHandler
	AIResponseDrafts        *AIResponseDraftHandler
	Proposals               *ProposalHandler
	ProposalExports         *ProposalExportHandler
	AIProposalDrafts        *AIProposalDraftHandler
	ProposalApplication     *ProposalApplicationHandler
	RequirementCorrections  *RequirementCorrectionHandler
	ReviewThreads           *ReviewThreadHandler
	InternalReviews         *InternalReviewDecisionHandler
	CollaborationTimeline   *CollaborationTimelineHandler
	ProposalPackages        *ProposalPackageHandler
	ProposalApprovals       *ProposalApprovalHandler
	Submissions             *SubmissionHandler
	ReportedOutcomes        *ReportedOutcomeHandler
	ExternalActionEvidence  *ExternalActionEvidenceHandler
	ArchivePreparation      *ArchivePreparationHandler
	BidArchive              *BidArchiveHandler
	OwnershipContinuity     *OwnershipContinuityHandler
	GoogleStart             *GoogleStartHandler
	GoogleCallback          *GoogleCallbackHandler
	GoogleLinkStart         *GoogleLinkStartHandler
	GoogleLinkCallback      *GoogleLinkCallbackHandler
}

func NewPilot(state *health.State, routes PilotRoutes) (*fiber.App, error) {
	if state == nil || routes.Browser == nil || routes.Login == nil || routes.PasswordChange == nil || routes.Logout == nil || routes.Bootstrap == nil || routes.SessionInventory == nil || routes.SessionAdministration == nil || routes.AccountType == nil || routes.RecoveryPreparation == nil || routes.MFAReadiness == nil || routes.MFA == nil || routes.Capture == nil || routes.OpportunityWorkflow == nil || routes.OpportunityRead == nil || routes.Requirements == nil || routes.DocumentUploads == nil || routes.AIRequirementReviews == nil || routes.RequirementEvidence == nil || routes.TechnicalResponses == nil || routes.AIResponseDrafts == nil || routes.Proposals == nil || routes.ProposalExports == nil || routes.AIProposalDrafts == nil || routes.ProposalApplication == nil || routes.RequirementCorrections == nil || routes.ReviewThreads == nil || routes.InternalReviews == nil || routes.CollaborationTimeline == nil || routes.ProposalPackages == nil || routes.ProposalApprovals == nil || routes.Submissions == nil || routes.ReportedOutcomes == nil || routes.ExternalActionEvidence == nil || routes.ArchivePreparation == nil || routes.BidArchive == nil || routes.OwnershipContinuity == nil || routes.Memberships == nil || routes.MembershipRevoke == nil || routes.MembershipOffboarding == nil {
		return nil, errors.New("pilot_http_routes_invalid")
	}
	app := newBase(opportunitytransport.CaptureBodyLimit)
	app.Use("/v1", routes.Browser.CaptureAuditContext)
	ready := func(c fiber.Ctx) error {
		if !state.Ready() {
			return fail(c, 503, "SERVICE_NOT_READY", "The service is not ready to accept requests.", true)
		}
		return c.Next()
	}
	app.Post(LoginPath, ready, routes.Login.Handle)
	app.Post(PasswordChangePath, ready, routes.Browser.Handle, routes.PasswordChange.Handle)
	if routes.PasswordRecovery != nil {
		app.Post(PasswordRecoveryRequestPath, ready, routes.PasswordRecovery.Request)
		app.Post(PasswordRecoveryResetPath, ready, routes.PasswordRecovery.Reset)
	}
	if routes.Registration != nil || routes.EmailVerification != nil || routes.EmailVerificationResend != nil {
		if routes.Registration == nil || routes.EmailVerification == nil || routes.EmailVerificationResend == nil {
			return nil, errors.New("pilot_registration_routes_invalid")
		}
		app.Post(RegistrationPath, ready, routes.Registration.Handle)
		app.Post(EmailVerificationPath, ready, routes.EmailVerification.Handle)
		app.Post(EmailVerificationResendPath, ready, routes.EmailVerificationResend.Handle)
	}
	if routes.InvitationIssue != nil || routes.InvitationAccept != nil || routes.InvitationDirectory != nil || routes.InvitationCancel != nil {
		if routes.InvitationIssue == nil || routes.InvitationAccept == nil || routes.InvitationDirectory == nil || routes.InvitationCancel == nil {
			return nil, errors.New("pilot_invitation_routes_invalid")
		}
		app.Post(InvitationIssuePath, ready, routes.Browser.Handle, routes.InvitationIssue.Handle)
		app.Post(InvitationAcceptPath, ready, routes.Browser.Handle, routes.InvitationAccept.Handle)
		app.Get(InvitationDirectoryPath, ready, routes.Browser.Handle, routes.InvitationDirectory.Handle)
		app.Delete(InvitationPath, ready, routes.Browser.Handle, routes.InvitationCancel.Handle)
	}
	if routes.GoogleStart != nil || routes.GoogleCallback != nil {
		if routes.GoogleStart == nil || routes.GoogleCallback == nil {
			return nil, errors.New("pilot_google_routes_invalid")
		}
		app.Post(GoogleStartPath, ready, routes.GoogleStart.Handle)
		app.Get(GoogleCallbackPath, ready, routes.GoogleCallback.Handle)
	}
	if routes.GoogleLinkStart != nil || routes.GoogleLinkCallback != nil {
		if routes.GoogleLinkStart == nil || routes.GoogleLinkCallback == nil || routes.Browser == nil {
			return nil, errors.New("pilot_google_link_routes_invalid")
		}
		app.Post(GoogleLinkStartPath, ready, routes.Browser.Handle, routes.GoogleLinkStart.Handle)
		app.Get(GoogleLinkCallbackPath, ready, routes.Browser.Handle, routes.GoogleLinkCallback.Handle)
	}
	app.Get(SessionBootstrapPath, ready, routes.Browser.Bootstrap, routes.Bootstrap.Handle)
	app.Get(SessionInventoryPath, ready, routes.Browser.Handle, routes.SessionInventory.Get)
	app.Delete(SessionRevokePath, ready, routes.Browser.NoTouch, routes.SessionAdministration.RevokeOther)
	app.Post(SessionRevokeAllPath, ready, routes.Browser.NoTouch, routes.SessionAdministration.RevokeAll)
	app.Get(MePortalPath, ready, routes.Browser.Handle, routes.AccountType.Get)
	app.Post(AccountTypeSelectionPath, ready, routes.Browser.Handle, routes.AccountType.Select)
	app.Get(RecoveryPreparationPath, ready, routes.Browser.Handle, routes.RecoveryPreparation.Get)
	app.Get(MFAReadinessPath, ready, routes.Browser.Handle, routes.MFAReadiness.Get)
	app.Post(MFAChallengePath, ready, routes.MFA.Challenge)
	app.Post(EmailLoginResendPath, ready, routes.MFA.ResendLoginCode)
	app.Post(EmailLoginCancelPath, ready, routes.MFA.CancelLoginCode)
	if !routes.MFA.emailOnly {
		app.Post(MFAEnrollPath, ready, routes.Browser.NoTouch, routes.MFA.Enroll)
		app.Post(MFAConfirmPath, ready, routes.Browser.NoTouch, routes.MFA.Confirm)
		app.Post(MFAStepUpPath, ready, routes.Browser.NoTouch, routes.MFA.StepUp)
		app.Post(MFARecoveryRegeneratePath, ready, routes.Browser.NoTouch, routes.MFA.Regenerate)
		app.Post(MFAReplacePath, ready, routes.Browser.NoTouch, routes.MFA.Replace)
	}
	app.Get(MembershipDirectoryPath, ready, routes.Browser.Handle, routes.Memberships.Handle)
	app.Get(MembershipDirectoryManagementPath, ready, routes.Browser.Handle, routes.Memberships.HandleManaged)
	app.Delete(MembershipPath, ready, routes.Browser.Handle, routes.MembershipRevoke.Handle)
	app.Post(MembershipOffboardingPath, ready, routes.Browser.Handle, routes.MembershipOffboarding.Begin)
	app.Get(MembershipOffboardingOperationPath, ready, routes.Browser.Handle, routes.MembershipOffboarding.Get)
	app.Post(MembershipOffboardingRetargetPath, ready, routes.Browser.Handle, routes.MembershipOffboarding.Retarget)
	app.Post(MembershipOffboardingCancelPath, ready, routes.Browser.Handle, routes.MembershipOffboarding.Cancel)
	app.Post(ManualCapturePath, ready, routes.Browser.Handle, routes.Capture.Handle)
	app.Post(OpportunityVerificationPath, ready, routes.Browser.Handle, routes.OpportunityWorkflow.Verify)
	app.Post(WorkspaceCreationPath, ready, routes.Browser.Handle, routes.OpportunityWorkflow.CreateWorkspace)
	app.Post(BidDecisionPath, ready, routes.Browser.Handle, routes.OpportunityWorkflow.RecordDecision)
	app.Get(OpportunityListPath, ready, routes.Browser.Handle, routes.OpportunityRead.ListOpportunities)
	app.Get(OpportunityDetailPath, ready, routes.Browser.Handle, routes.OpportunityRead.GetOpportunity)
	app.Get(BidListPath, ready, routes.Browser.Handle, routes.OpportunityRead.ListBids)
	app.Get(BidDetailPath, ready, routes.Browser.Handle, routes.OpportunityRead.GetBid)
	app.Post(RequirementListPath, ready, routes.Browser.Handle, routes.Requirements.Create)
	app.Get(RequirementListPath, ready, routes.Browser.Handle, routes.Requirements.List)
	app.Patch(RequirementAssignmentPath, ready, routes.Browser.Handle, routes.Requirements.Assign)
	app.Get(ReadinessPath, ready, routes.Browser.Handle, routes.Requirements.Readiness)
	app.Post(DocumentUploadInitiatePath, ready, routes.Browser.Handle, routes.DocumentUploads.Initiate)
	app.Post(DocumentUploadCompletePath, ready, routes.Browser.Handle, routes.DocumentUploads.Complete)
	app.Get(DocumentJobPath, ready, routes.Browser.Handle, routes.DocumentUploads.Job)
	app.Get(AIRequirementListPath, ready, routes.Browser.Handle, routes.AIRequirementReviews.List)
	app.Post(AIRequirementAcceptPath, ready, routes.Browser.Handle, routes.AIRequirementReviews.Accept)
	app.Post(AIRequirementRejectPath, ready, routes.Browser.Handle, routes.AIRequirementReviews.Reject)
	app.Post(EvidenceUploadInitiatePath, ready, routes.Browser.Handle, routes.RequirementEvidence.Initiate)
	app.Post(EvidenceUploadCompletePath, ready, routes.Browser.Handle, routes.RequirementEvidence.Complete)
	app.Post(RequirementSatisfyPath, ready, routes.Browser.Handle, routes.RequirementEvidence.Satisfy)
	app.Get(RequirementEvidenceListPath, ready, routes.Browser.Handle, routes.RequirementEvidence.List)
	app.Put(TechnicalResponsePath, ready, routes.Browser.Handle, routes.TechnicalResponses.Save)
	app.Get(TechnicalResponsePath, ready, routes.Browser.Handle, routes.TechnicalResponses.Get)
	app.Get(TechnicalResponseHistoryPath, ready, routes.Browser.Handle, routes.TechnicalResponses.History)
	app.Get(GapReviewPath, ready, routes.Browser.Handle, routes.TechnicalResponses.GapReview)
	app.Post(AIResponseDraftPath, ready, routes.Browser.Handle, routes.AIResponseDrafts.Request)
	app.Get(AIResponseDraftPath, ready, routes.Browser.Handle, routes.AIResponseDrafts.List)
	app.Put(ProposalDocumentPath, ready, routes.Browser.Handle, routes.Proposals.Save)
	app.Get(ProposalDocumentPath, ready, routes.Browser.Handle, routes.Proposals.Get)
	app.Get(ProposalHistoryPath, ready, routes.Browser.Handle, routes.Proposals.History)
	app.Get(ProposalExportPath, ready, routes.Browser.Handle, routes.ProposalExports.Download)
	app.Post(AIProposalDraftPath, ready, routes.Browser.Handle, routes.AIProposalDrafts.Request)
	app.Get(AIProposalDraftPath, ready, routes.Browser.Handle, routes.AIProposalDrafts.List)
	app.Post(AIProposalApplyPath, ready, routes.Browser.Handle, routes.ProposalApplication.Apply)
	app.Get(ProposalApprovalReadinessPath, ready, routes.Browser.Handle, routes.ProposalApplication.Readiness)
	app.Patch(RequirementCorrectionPath, ready, routes.Browser.Handle, routes.RequirementCorrections.Correct)
	app.Get(RequirementCorrectionHistoryPath, ready, routes.Browser.Handle, routes.RequirementCorrections.History)
	app.Post(ReviewThreadListPath, ready, routes.Browser.Handle, routes.ReviewThreads.Create)
	app.Get(ReviewThreadListPath, ready, routes.Browser.Handle, routes.ReviewThreads.List)
	app.Post(ReviewThreadCommentPath, ready, routes.Browser.Handle, routes.ReviewThreads.Comment)
	app.Post(ReviewThreadResolvePath, ready, routes.Browser.Handle, routes.ReviewThreads.Resolve)
	app.Patch(ReviewThreadAssignmentPath, ready, routes.Browser.Handle, routes.ReviewThreads.Assign)
	app.Get(ReviewThreadAssignmentHistoryPath, ready, routes.Browser.Handle, routes.ReviewThreads.AssignmentHistory)
	app.Get(ReviewDashboardPath, ready, routes.Browser.Handle, routes.ReviewThreads.Dashboard)
	app.Get(ReviewWorkloadsPath, ready, routes.Browser.Handle, routes.ReviewThreads.Workloads)
	app.Get(ReviewerOpenWorkPath, ready, routes.Browser.Handle, routes.ReviewThreads.ReviewerWork)
	app.Post(InternalReviewDecisionPath, ready, routes.Browser.Handle, routes.InternalReviews.Record)
	app.Get(InternalReviewDecisionPath, ready, routes.Browser.Handle, routes.InternalReviews.History)
	app.Get(ApprovalPolicyPreparationPath, ready, routes.Browser.Handle, routes.InternalReviews.Preparation)
	app.Get(CollaborationTimelinePath, ready, routes.Browser.Handle, routes.CollaborationTimeline.List)
	app.Post(ProposalPackagePreparationPath, ready, routes.Browser.Handle, routes.ProposalPackages.Prepare)
	app.Get(ProposalPackagePreparationPath, ready, routes.Browser.Handle, routes.ProposalPackages.List)
	app.Get(ProposalPackageManifestPath, ready, routes.Browser.Handle, routes.ProposalPackages.Get)
	app.Post(ProposalApprovalPath, ready, routes.Browser.Handle, routes.ProposalApprovals.Approve)
	app.Get(ProposalApprovalPath, ready, routes.Browser.Handle, routes.ProposalApprovals.Get)
	app.Post(SubmissionProofUploadsPath, ready, routes.Browser.Handle, routes.Submissions.Initiate)
	app.Post(SubmissionProofCompletePath, ready, routes.Browser.Handle, routes.Submissions.Complete)
	app.Post(SubmissionConfirmationPath, ready, routes.Browser.Handle, routes.Submissions.Confirm)
	app.Get(SubmissionConfirmationPath, ready, routes.Browser.Handle, routes.Submissions.Get)
	app.Post(OutcomeProofUploadsPath, ready, routes.Browser.Handle, routes.ReportedOutcomes.Initiate)
	app.Post(OutcomeProofCompletePath, ready, routes.Browser.Handle, routes.ReportedOutcomes.Complete)
	app.Post(ReportedOutcomesPath, ready, routes.Browser.Handle, routes.ReportedOutcomes.Report)
	app.Get(ReportedOutcomesPath, ready, routes.Browser.Handle, routes.ReportedOutcomes.History)
	app.Get(CurrentReportedOutcomePath, ready, routes.Browser.Handle, routes.ReportedOutcomes.Current)
	app.Post(ExternalActionEvidencePath, ready, routes.Browser.Handle, routes.ExternalActionEvidence.Report)
	app.Get(ExternalActionEvidencePath, ready, routes.Browser.Handle, routes.ExternalActionEvidence.List)
	app.Get(ExternalActionEvidenceDetailPath, ready, routes.Browser.Handle, routes.ExternalActionEvidence.Get)
	app.Get(ExternalEvidenceReconciliationPath, ready, routes.Browser.Handle, routes.ExternalActionEvidence.Reconciliation)
	app.Get(ArchivePreparationPath, ready, routes.Browser.Handle, routes.ArchivePreparation.Get)
	app.Post(BidArchivePath, ready, routes.Browser.Handle, routes.BidArchive.Archive)
	app.Get(BidArchivePath, ready, routes.Browser.Handle, routes.BidArchive.Get)
	app.Get(OwnershipContinuityPath, ready, routes.Browser.Handle, routes.OwnershipContinuity.Get)
	app.Post(LogoutPath, ready, routes.Browser.Handle, routes.Logout.Handle)
	installHealthFallback(app, state)
	return app, nil
}
