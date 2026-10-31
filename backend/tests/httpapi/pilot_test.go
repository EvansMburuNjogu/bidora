package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bidora/backend/internal/health"
	"bidora/backend/internal/httpapi"
)

func emptyPilotRoutes() httpapi.PilotRoutes {
	return httpapi.PilotRoutes{
		Browser:                &httpapi.BrowserSessionMiddleware{},
		Login:                  &httpapi.LoginHandler{},
		PasswordChange:         &httpapi.PasswordChangeHandler{},
		Logout:                 &httpapi.LogoutHandler{},
		Bootstrap:              &httpapi.SessionBootstrapHandler{},
		SessionInventory:       &httpapi.SessionInventoryHandler{},
		SessionAdministration:  &httpapi.SessionAdministrationHandler{},
		AccountType:            &httpapi.AccountTypeHandler{},
		RecoveryPreparation:    &httpapi.RecoveryPreparationHandler{},
		MFAReadiness:           &httpapi.MFAReadinessHandler{},
		MFA:                    &httpapi.MFAHandler{},
		Capture:                &httpapi.ManualCaptureHandler{},
		OpportunityWorkflow:    &httpapi.OpportunityWorkflowHandler{},
		OpportunityRead:        &httpapi.OpportunityReadHandler{},
		Requirements:           &httpapi.RequirementHandler{},
		DocumentUploads:        &httpapi.DocumentUploadHandler{},
		AIRequirementReviews:   &httpapi.AIRequirementReviewHandler{},
		RequirementEvidence:    &httpapi.RequirementEvidenceHandler{},
		TechnicalResponses:     &httpapi.TechnicalResponseHandler{},
		AIResponseDrafts:       &httpapi.AIResponseDraftHandler{},
		Proposals:              &httpapi.ProposalHandler{},
		ProposalExports:        &httpapi.ProposalExportHandler{},
		AIProposalDrafts:       &httpapi.AIProposalDraftHandler{},
		ProposalApplication:    &httpapi.ProposalApplicationHandler{},
		RequirementCorrections: &httpapi.RequirementCorrectionHandler{},
		ReviewThreads:          &httpapi.ReviewThreadHandler{},
		InternalReviews:        &httpapi.InternalReviewDecisionHandler{},
		CollaborationTimeline:  &httpapi.CollaborationTimelineHandler{},
		ProposalPackages:       &httpapi.ProposalPackageHandler{},
		ProposalApprovals:      &httpapi.ProposalApprovalHandler{},
		Submissions:            &httpapi.SubmissionHandler{},
		ReportedOutcomes:       &httpapi.ReportedOutcomeHandler{},
		ExternalActionEvidence: &httpapi.ExternalActionEvidenceHandler{},
		ArchivePreparation:     &httpapi.ArchivePreparationHandler{},
		BidArchive:             &httpapi.BidArchiveHandler{},
		OwnershipContinuity:    &httpapi.OwnershipContinuityHandler{},
		Memberships:            &httpapi.MembershipDirectoryHandler{},
		MembershipRevoke:       &httpapi.MembershipRevocationHandler{},
		MembershipOffboarding:  &httpapi.MembershipOffboardingHandler{},
	}
}
func TestPilotHTTPFactoryRequiresCompleteDependencies(t *testing.T) {
	state := &health.State{}
	valid := emptyPilotRoutes()
	cases := []struct {
		name   string
		state  *health.State
		routes httpapi.PilotRoutes
	}{{"state", nil, valid}, {"browser", state, func() httpapi.PilotRoutes { v := valid; v.Browser = nil; return v }()}, {"login", state, func() httpapi.PilotRoutes { v := valid; v.Login = nil; return v }()}, {"password change", state, func() httpapi.PilotRoutes { v := valid; v.PasswordChange = nil; return v }()}, {"logout", state, func() httpapi.PilotRoutes { v := valid; v.Logout = nil; return v }()}, {"bootstrap", state, func() httpapi.PilotRoutes { v := valid; v.Bootstrap = nil; return v }()}, {"capture", state, func() httpapi.PilotRoutes { v := valid; v.Capture = nil; return v }()}, {"memberships", state, func() httpapi.PilotRoutes { v := valid; v.Memberships = nil; return v }()}, {"membership revoke", state, func() httpapi.PilotRoutes { v := valid; v.MembershipRevoke = nil; return v }()}}
	for _, tc := range cases {
		if app, err := httpapi.NewPilot(tc.state, tc.routes); err == nil || app != nil {
			t.Fatal("invalid pilot routes accepted", tc.name)
		}
	}
	partialRegistration := valid
	partialRegistration.Registration = &httpapi.RegistrationHandler{}
	if app, err := httpapi.NewPilot(state, partialRegistration); err == nil || app != nil {
		t.Fatal("partial registration route pair accepted")
	}
	partialInvitation := valid
	partialInvitation.InvitationIssue = &httpapi.InvitationIssueHandler{}
	if app, err := httpapi.NewPilot(state, partialInvitation); err == nil || app != nil {
		t.Fatal("partial invitation route pair accepted")
	}
	partialInvitationDirectory := valid
	partialInvitationDirectory.InvitationDirectory = &httpapi.InvitationDirectoryHandler{}
	if app, err := httpapi.NewPilot(state, partialInvitationDirectory); err == nil || app != nil {
		t.Fatal("partial invitation directory route set accepted")
	}
	partialGoogle := valid
	partialGoogle.GoogleStart = &httpapi.GoogleStartHandler{}
	if app, err := httpapi.NewPilot(state, partialGoogle); err == nil || app != nil {
		t.Fatal("partial Google route pair accepted")
	}
	partialLink := valid
	partialLink.GoogleLinkStart = &httpapi.GoogleLinkStartHandler{}
	if app, err := httpapi.NewPilot(state, partialLink); err == nil || app != nil {
		t.Fatal("partial Google link route pair accepted")
	}
}
func TestPilotHTTPFactoryGatesBusinessRoutesAndPreservesHealth(t *testing.T) {
	state := &health.State{}
	app, err := httpapi.NewPilot(state, emptyPilotRoutes())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{httpapi.LoginPath, httpapi.LogoutPath, httpapi.ManualCapturePath} {
		req := httptest.NewRequest("POST", path, nil)
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 503 {
			t.Fatal("unready pilot route accepted", path, res.StatusCode)
		}
	}
	bootstrap, err := app.Test(httptest.NewRequest("GET", httpapi.SessionBootstrapPath, nil))
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Body.Close()
	if bootstrap.StatusCode != 503 {
		t.Fatal("unready bootstrap route accepted", bootstrap.StatusCode)
	}
	live, err := app.Test(httptest.NewRequest("GET", "/health/live", nil))
	if err != nil {
		t.Fatal(err)
	}
	live.Body.Close()
	if live.StatusCode != 200 {
		t.Fatal("pilot liveness unavailable")
	}
	state.Start()
	ready, err := app.Test(httptest.NewRequest("GET", "/health/ready", nil))
	if err != nil {
		t.Fatal(err)
	}
	ready.Body.Close()
	if ready.StatusCode != 200 {
		t.Fatal("pilot readiness unavailable")
	}
	unknown, err := app.Test(httptest.NewRequest("GET", "/unknown", nil))
	if err != nil {
		t.Fatal(err)
	}
	unknown.Body.Close()
	if unknown.StatusCode != 404 {
		t.Fatal("pilot fallback drifted")
	}
}
func TestPilotHTTPFactoryRegistersConfiguredRegistrationBehindReadiness(t *testing.T) {
	state := &health.State{}
	routes := emptyPilotRoutes()
	routes.Registration = &httpapi.RegistrationHandler{}
	routes.EmailVerification = &httpapi.EmailVerificationHandler{}
	routes.EmailVerificationResend = &httpapi.EmailVerificationResendHandler{}
	app, err := httpapi.NewPilot(state, routes)
	if err != nil {
		t.Fatal(err)
	}
	response, err := app.Test(httptest.NewRequest(http.MethodPost, httpapi.RegistrationPath, nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("unready registration route accepted", response.StatusCode)
	}
	verification, err := app.Test(httptest.NewRequest(http.MethodPost, httpapi.EmailVerificationPath, nil))
	if err != nil {
		t.Fatal(err)
	}
	verification.Body.Close()
	if verification.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("unready verification route accepted", verification.StatusCode)
	}
	resend, err := app.Test(httptest.NewRequest(http.MethodPost, httpapi.EmailVerificationResendPath, nil))
	if err != nil {
		t.Fatal(err)
	}
	resend.Body.Close()
	if resend.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("unready verification resend route accepted", resend.StatusCode)
	}
}
func TestPilotHTTPFactoryRegistersConfiguredInvitationRoutesBehindReadiness(t *testing.T) {
	state := &health.State{}
	routes := emptyPilotRoutes()
	routes.InvitationIssue = &httpapi.InvitationIssueHandler{}
	routes.InvitationAccept = &httpapi.InvitationAcceptHandler{}
	routes.InvitationDirectory = &httpapi.InvitationDirectoryHandler{}
	routes.InvitationCancel = &httpapi.InvitationCancellationHandler{}
	app, err := httpapi.NewPilot(state, routes)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{httpapi.InvitationIssuePath, httpapi.InvitationAcceptPath} {
		response, requestErr := app.Test(httptest.NewRequest(http.MethodPost, path, nil))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusServiceUnavailable {
			t.Fatal("unready invitation route accepted", path, response.StatusCode)
		}
	}
	directory, requestErr := app.Test(httptest.NewRequest(http.MethodGet, httpapi.InvitationDirectoryPath, nil))
	if requestErr != nil {
		t.Fatal(requestErr)
	}
	directory.Body.Close()
	if directory.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("unready invitation directory accepted", directory.StatusCode)
	}
	cancellation, requestErr := app.Test(httptest.NewRequest(http.MethodDelete, "/v1/shared/invitations/"+"00000000-0000-0000-0000-000000000001", nil))
	if requestErr != nil {
		t.Fatal(requestErr)
	}
	cancellation.Body.Close()
	if cancellation.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("unready invitation cancellation accepted", cancellation.StatusCode)
	}
}
func TestPilotHTTPFactoryRegistersConfiguredGoogleRoutesBehindReadiness(t *testing.T) {
	state := &health.State{}
	routes := emptyPilotRoutes()
	routes.GoogleStart = &httpapi.GoogleStartHandler{}
	routes.GoogleCallback = &httpapi.GoogleCallbackHandler{}
	routes.GoogleLinkStart = &httpapi.GoogleLinkStartHandler{}
	routes.GoogleLinkCallback = &httpapi.GoogleLinkCallbackHandler{}
	app, err := httpapi.NewPilot(state, routes)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []*http.Request{httptest.NewRequest("POST", httpapi.GoogleStartPath, nil), httptest.NewRequest("GET", httpapi.GoogleCallbackPath, nil), httptest.NewRequest("POST", httpapi.GoogleLinkStartPath, nil), httptest.NewRequest("GET", httpapi.GoogleLinkCallbackPath, nil)} {
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 503 {
			t.Fatal("unready Google route accepted", request.URL.Path, response.StatusCode)
		}
	}
}

func TestPilotHTTPFactoryEnforcesCaptureBodyCeiling(t *testing.T) {
	state := &health.State{}
	state.Start()
	app, err := httpapi.NewPilot(state, emptyPilotRoutes())
	if err != nil {
		t.Fatal(err)
	}
	if app.Config().BodyLimit != 32*1024 {
		t.Fatal("pilot body ceiling drifted", app.Config().BodyLimit)
	}
}
