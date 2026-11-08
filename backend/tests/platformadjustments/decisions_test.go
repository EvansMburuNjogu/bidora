package platformadjustments_test

import (
	app "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	"context"
	"errors"
	"testing"
	"time"
)

type ids struct{ next byte }

func (g *ids) NewID() (domain.ID, error) { g.next++; return domain.ID{g.next}, nil }

type clock struct{ at time.Time }

func (c clock) Now() time.Time { return c.at }

type store struct {
	app.AdjustmentTx
	at        time.Time
	actor     domain.ID
	owner     bool
	billing   bool
	stored    app.AdjustmentStored
	committed *app.AdjustmentDecision
}

func (s *store) WithinAdjustment(c context.Context, fn func(context.Context, app.AdjustmentTx) error) error {
	return fn(c, s)
}
func (s *store) ResolveAdjustmentInvoiceTenant(context.Context, domain.ID) (domain.ID, bool, error) {
	return domain.ID{7}, true, nil
}
func (s *store) ResolveAdjustmentChangeTenant(context.Context, domain.ID) (domain.ID, bool, error) {
	return domain.ID{7}, true, nil
}
func (s *store) ResolveAdjustmentAuthority(_ context.Context, _ app.AdjustmentActor, _ domain.ID, _ time.Time) (app.AdjustmentAuthority, bool, error) {
	return app.AdjustmentAuthority{UserID: s.actor, ContextGeneration: 2, AdministratorVersion: 1, PlatformOwner: s.owner, BillingAdministrator: s.billing, AssuranceMethod: "AAL2:TOTP", AssuredAt: s.at.Add(-time.Minute)}, false, nil
}
func (s *store) GetAdjustmentCommand(context.Context, domain.ID, string, [32]byte) (app.AdjustmentCommand, bool, error) {
	return app.AdjustmentCommand{}, false, nil
}
func (s *store) LockAdjustment(context.Context, domain.ID, domain.ID) (app.AdjustmentStored, bool, error) {
	return s.stored, true, nil
}
func (s *store) LockCreditNoteNumber(context.Context, int, time.Time) (int64, error) { return 1, nil }
func (s *store) CommitAdjustmentDecision(_ context.Context, d app.AdjustmentDecision, _ app.AdjustmentStored, _ app.AdjustmentEvidence) error {
	s.committed = &d
	return nil
}
func TestAdjustmentProposalRequiresBillingRoleNotOwnerAlone(t *testing.T) {
	at := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := &store{at: at, actor: domain.ID{41}, owner: true}
	svc, _ := app.NewAdjustmentService(s, &ids{}, clock{at}, app.AdjustmentPolicy{RefundRequestWindow: domain.RefundEligibilityWindow})
	in := app.ProposeAdjustmentInput{Actor: app.AdjustmentActor{UserID: s.actor, SessionID: domain.ID{3}, ContextGeneration: 2}, InvoiceID: domain.ID{11}, CorrelationID: domain.ID{4}, Action: "ISSUE_CREDIT", ReasonCode: "SERVICE_CREDIT", AmountMinorUnits: 100, EntitlementDisposition: "NO_ENTITLEMENT_CHANGE", ProposalReason: "Reviewed", ExpectedInvoiceVersion: 1, ExpectedPaymentVersion: 1, IdempotencyKey: "test-key"}
	if _, e := svc.Propose(context.Background(), in); !errors.Is(e, app.ErrAdjustmentNotPermitted) {
		t.Fatal("owner-only proposal allowed", e)
	}
}
func TestRefundProposalFailsClosedWithoutApprovedContractWindow(t *testing.T) {
	at := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := &store{at: at, actor: domain.ID{41}, billing: true}
	svc, e := app.NewAdjustmentService(s, &ids{}, clock{at}, app.AdjustmentPolicy{})
	if e != nil {
		t.Fatal("zero window should preserve credit-only source readiness", e)
	}
	in := app.ProposeAdjustmentInput{Actor: app.AdjustmentActor{UserID: s.actor, SessionID: domain.ID{3}, ContextGeneration: 2}, InvoiceID: domain.ID{11}, CorrelationID: domain.ID{4}, Action: "REQUEST_REFUND", ReasonCode: "SERVICE_FAILURE_REFUND", AmountMinorUnits: 100, EntitlementDisposition: "REVIEW_REQUIRED", ProposalReason: "Reviewed", ExpectedInvoiceVersion: 1, ExpectedPaymentVersion: 1, IdempotencyKey: "test-key"}
	if _, e = svc.Propose(context.Background(), in); !errors.Is(e, app.ErrAdjustmentNotPermitted) {
		t.Fatal("unapproved refund window allowed proposal", e)
	}
	if _, e = app.NewAdjustmentService(s, &ids{}, clock{at}, app.AdjustmentPolicy{RefundRequestWindow: domain.RefundEligibilityWindow + time.Hour}); e == nil {
		t.Fatal("window beyond maximum accepted")
	}
}
func TestAdjustmentMakerCheckerAndPendingRefundOutcome(t *testing.T) {
	at := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	maker := domain.ID{40}
	checker := domain.ID{41}
	proposal := domain.AdjustmentProposal{ID: domain.ID{10}, TenantID: domain.ID{7}, InvoiceID: domain.ID{11}, PaymentID: domain.ID{12}, MakerUserID: maker, Action: "REQUEST_REFUND", ReasonCode: "SERVICE_FAILURE_REFUND", AmountMinorUnits: 1250, NetMinorUnits: 1000, TaxMinorUnits: 250, ExpectedInvoiceVersion: 3, ExpectedPaymentVersion: 5, ExpiresAt: at.Add(30 * time.Minute), Version: 1}
	s := &store{at: at, actor: maker, owner: true, stored: app.AdjustmentStored{Proposal: proposal, State: "PENDING_APPROVAL", InvoiceState: "PAID", PaymentState: "SETTLED", CurrentInvoiceVersion: 3, CurrentPaymentVersion: 5, InvoiceTotalMinor: 1250, PaymentSettledAt: at.Add(-time.Hour)}}
	svc, e := app.NewAdjustmentService(s, &ids{}, clock{at}, app.AdjustmentPolicy{RefundRequestWindow: domain.RefundEligibilityWindow})
	if e != nil {
		t.Fatal(e)
	}
	in := app.DecideAdjustmentInput{Actor: app.AdjustmentActor{UserID: maker, SessionID: domain.ID{3}, ContextGeneration: 2}, ChangeID: proposal.ID, CorrelationID: domain.ID{4}, Decision: "APPROVED", Reason: "Independent review", ExpectedVersion: 1, IdempotencyKey: "test-key"}
	if _, e = svc.Decide(context.Background(), in); !errors.Is(e, app.ErrAdjustmentApprovalRequired) || s.committed != nil {
		t.Fatal("maker approved own adjustment", e)
	}
	s.actor = checker
	in.Actor.UserID = checker
	s.owner = false
	if _, e = svc.Decide(context.Background(), in); !errors.Is(e, app.ErrAdjustmentNotPermitted) || s.committed != nil {
		t.Fatal("non-owner approved adjustment", e)
	}
	s.owner = true
	disabled, _ := app.NewAdjustmentService(s, &ids{}, clock{at}, app.AdjustmentPolicy{})
	if _, e = disabled.Decide(context.Background(), in); !errors.Is(e, app.ErrAdjustmentNotPermitted) || s.committed != nil {
		t.Fatal("disabled policy approved pending refund", e)
	}
	s.stored.CurrentPaymentVersion = 6
	if _, e = svc.Decide(context.Background(), in); !errors.Is(e, app.ErrAdjustmentConflict) || s.committed != nil {
		t.Fatal("stale payment approved", e)
	}
	s.stored.CurrentPaymentVersion = 5
	s.stored.Proposal.ExpiresAt = at.Add(-time.Second)
	if _, e = svc.Decide(context.Background(), in); !errors.Is(e, app.ErrAdjustmentApprovalRequired) || s.committed != nil {
		t.Fatal("expired adjustment approved", e)
	}
	s.stored.Proposal.ExpiresAt = at.Add(30 * time.Minute)
	shortWindow, _ := app.NewAdjustmentService(s, &ids{}, clock{at}, app.AdjustmentPolicy{RefundRequestWindow: time.Minute})
	if _, e = shortWindow.Decide(context.Background(), in); !errors.Is(e, app.ErrAdjustmentApprovalRequired) || s.committed != nil {
		t.Fatal("approval after provider contract cutoff accepted", e)
	}
	s.stored.PaymentSettledAt = time.Time{}
	if _, e = svc.Decide(context.Background(), in); !errors.Is(e, app.ErrAdjustmentApprovalRequired) || s.committed != nil {
		t.Fatal("missing authoritative settlement accepted", e)
	}
	s.stored.PaymentSettledAt = at.Add(-time.Hour)
	result, e := svc.Decide(context.Background(), in)
	if e != nil || result.State != "APPROVED" || result.CreditNoteID.IsZero() || result.RefundID.IsZero() || s.committed == nil || s.committed.CreditState != "REFUND_PENDING" {
		t.Fatal("expected recorded pending refund, never provider settlement", result, e, s.committed)
	}
}
func TestAdjustmentRejectionDoesNotIssueCredit(t *testing.T) {
	at := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := &store{at: at, actor: domain.ID{41}, owner: true, stored: app.AdjustmentStored{Proposal: domain.AdjustmentProposal{ID: domain.ID{10}, TenantID: domain.ID{7}, InvoiceID: domain.ID{11}, PaymentID: domain.ID{12}, MakerUserID: domain.ID{40}, Action: "ISSUE_CREDIT", Version: 1, ExpiresAt: at.Add(time.Minute)}, State: "PENDING_APPROVAL"}}
	svc, _ := app.NewAdjustmentService(s, &ids{}, clock{at}, app.AdjustmentPolicy{RefundRequestWindow: domain.RefundEligibilityWindow})
	in := app.DecideAdjustmentInput{Actor: app.AdjustmentActor{UserID: s.actor, SessionID: domain.ID{3}, ContextGeneration: 2}, ChangeID: s.stored.Proposal.ID, CorrelationID: domain.ID{4}, Decision: "REJECTED", Reason: "Insufficient evidence", ExpectedVersion: 1, IdempotencyKey: "test-key"}
	result, e := svc.Decide(context.Background(), in)
	if e != nil || result.State != "REJECTED" || !result.CreditNoteID.IsZero() || !result.RefundID.IsZero() || s.committed == nil || s.committed.CreditNumber != "" {
		t.Fatal("rejection has financial effect", result, e)
	}
}
