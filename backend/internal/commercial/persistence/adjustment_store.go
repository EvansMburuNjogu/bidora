package persistence

import (
	"context"
	"errors"
	"time"

	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	commercialsql "bidora/backend/internal/commercial/persistence/sqlc"
	controlapp "bidora/backend/internal/controlplane/application"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformdomain "bidora/backend/internal/platformadmin/domain"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

func (s *Store) WithinAdjustment(ctx context.Context, fn func(context.Context, commercialapp.AdjustmentTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_adjustment_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := newTrialTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}

func (tx *trialTx) ResolveAdjustmentAuthority(ctx context.Context, actor commercialapp.AdjustmentActor, tenantID domain.ID, at time.Time) (commercialapp.AdjustmentAuthority, bool, error) {
	facts, tie, err := tx.financialAuthority.Resolve(ctx, platformapp.AuthorityQuery{UserID: platformapp.ID(actor.UserID), SessionID: platformapp.ID(actor.SessionID), ObservedAt: at}, platformapp.ID(tenantID))
	if errors.Is(err, platformapp.ErrAuthorityNotFound) {
		return commercialapp.AdjustmentAuthority{}, false, nil
	}
	if err != nil {
		return commercialapp.AdjustmentAuthority{}, false, err
	}
	billing, owner := false, false
	for _, r := range facts.Roles {
		billing = billing || r == platformdomain.BillingAdministrator
		owner = owner || r == platformdomain.PlatformOwner
	}
	return commercialapp.AdjustmentAuthority{UserID: domain.ID(facts.UserID), ContextGeneration: facts.ContextGeneration, AdministratorVersion: facts.AdministratorVersion, BillingAdministrator: billing, PlatformOwner: owner, InvoiceReader: facts.Grants("platform.invoice.read"), PaymentReader: facts.Grants("platform.payment.read"), AssuranceMethod: facts.AssuranceLevel + ":" + facts.AssuranceMethod, AssuredAt: facts.AssuredAt.UTC()}, tie, nil
}
func (tx *trialTx) ResolveAdjustmentInvoiceTenant(ctx context.Context, invoice domain.ID) (domain.ID, bool, error) {
	r, e := tx.commercial.ResolveAdjustmentInvoiceTenant(ctx, commercialID(invoice))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.ID{}, false, nil
	}
	if e != nil {
		return domain.ID{}, false, e
	}
	if !r.Valid {
		return domain.ID{}, false, errors.New("commercial_adjustment_tenant_corrupt")
	}
	return domain.ID(r.Bytes), true, nil
}
func (tx *trialTx) ResolveAdjustmentChangeTenant(ctx context.Context, change domain.ID) (domain.ID, bool, error) {
	r, e := tx.commercial.ResolveAdjustmentChangeTenant(ctx, commercialID(change))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.ID{}, false, nil
	}
	if e != nil {
		return domain.ID{}, false, e
	}
	if !r.Valid {
		return domain.ID{}, false, errors.New("commercial_adjustment_tenant_corrupt")
	}
	return domain.ID(r.Bytes), true, nil
}

func (tx *trialTx) GetAdjustmentCommand(ctx context.Context, actor domain.ID, operation string, key [32]byte) (commercialapp.AdjustmentCommand, bool, error) {
	r, e := tx.commercial.GetAdjustmentCommand(ctx, commercialsql.GetAdjustmentCommandParams{ActorUserID: commercialID(actor), Operation: operation, KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return commercialapp.AdjustmentCommand{}, false, nil
	}
	if e != nil {
		return commercialapp.AdjustmentCommand{}, false, e
	}
	h, ok := hash32(r.RequestHash)
	if !ok || !r.TargetID.Valid {
		return commercialapp.AdjustmentCommand{}, false, errors.New("commercial_adjustment_command_corrupt")
	}
	return commercialapp.AdjustmentCommand{RequestHash: h, TargetID: domain.ID(r.TargetID.Bytes), State: r.ResponseState, Version: r.ResponseVersion}, true, nil
}
func (tx *trialTx) GetAdjustment(ctx context.Context, tenant, change domain.ID, _ bool) (commercialapp.AdjustmentStored, bool, error) {
	return tx.lockAdjustment(ctx, tenant, change)
}
func (tx *trialTx) LockAdjustment(ctx context.Context, tenant, change domain.ID) (commercialapp.AdjustmentStored, bool, error) {
	return tx.lockAdjustment(ctx, tenant, change)
}
func (tx *trialTx) lockAdjustment(ctx context.Context, tenant, change domain.ID) (commercialapp.AdjustmentStored, bool, error) {
	r, e := tx.commercial.LockFinancialChange(ctx, commercialsql.LockFinancialChangeParams{ChangeID: commercialID(change), TenantID: commercialID(tenant)})
	if errors.Is(e, pgx.ErrNoRows) {
		return commercialapp.AdjustmentStored{}, false, nil
	}
	if e != nil {
		return commercialapp.AdjustmentStored{}, false, e
	}
	tax, maker, request := mustHashes(r.TaxProfileHash, r.MakerAuthorityHash, r.RequestHash)
	if tax == [32]byte{} || maker == [32]byte{} || request == [32]byte{} || !r.PaymentID.Valid {
		return commercialapp.AdjustmentStored{}, false, errors.New("commercial_adjustment_corrupt")
	}
	p := domain.AdjustmentProposal{ID: domain.ID(r.ID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), PaymentID: domain.ID(r.PaymentID.Bytes), MakerUserID: domain.ID(r.MakerUserID.Bytes), Action: r.Action, ReasonCode: r.ReasonCode, EntitlementDisposition: r.EntitlementDisposition, ProposalReason: r.ProposalReason, AmountMinorUnits: r.AmountMinorUnits, NetMinorUnits: r.NetMinorUnits, TaxMinorUnits: r.TaxMinorUnits, ExpectedInvoiceVersion: r.ExpectedInvoiceVersion, ExpectedPaymentVersion: r.ExpectedPaymentVersion, TaxProfileID: domain.ID(r.TaxProfileID.Bytes), TaxProfileHash: tax, MakerAuthorityHash: maker, RequestHash: request, Version: r.Version, CreatedAt: r.CreatedAt.Time.UTC(), ExpiresAt: r.ExpiresAt.Time.UTC()}
	return commercialapp.AdjustmentStored{Proposal: p, State: r.State, InvoiceState: r.InvoiceState, PaymentState: r.PaymentState, CheckerUserID: domain.ID(r.CheckerUserID.Bytes), CreditNoteID: domain.ID(r.CreditNoteID.Bytes), RefundID: domain.ID(r.RefundID.Bytes), CreditNumber: r.CreditNumber, CurrentInvoiceVersion: r.InvoiceVersion, CurrentPaymentVersion: r.PaymentVersion, InvoiceTotalMinor: r.TotalMinorUnits, PriorCreditMinor: r.PriorCreditMinorUnits}, true, nil
}
func (tx *trialTx) ExpirePendingAdjustment(ctx context.Context, tenant, invoice domain.ID, at time.Time) (commercialapp.ExpiredAdjustment, bool, error) {
	r, err := tx.commercial.ExpirePendingFinancialChange(ctx, commercialsql.ExpirePendingFinancialChangeParams{At: commercialTime(at), TenantID: commercialID(tenant), InvoiceID: commercialID(invoice)})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.ExpiredAdjustment{}, false, nil
	}
	if err != nil {
		return commercialapp.ExpiredAdjustment{}, false, err
	}
	if !r.PaymentID.Valid || r.Version < 2 || r.AmountMinorUnits < 1 {
		return commercialapp.ExpiredAdjustment{}, false, errors.New("commercial_expired_adjustment_corrupt")
	}
	return commercialapp.ExpiredAdjustment{ChangeID: domain.ID(r.ID.Bytes), PaymentID: domain.ID(r.PaymentID.Bytes), Action: r.Action, ReasonCode: r.ReasonCode, EntitlementDisposition: r.EntitlementDisposition, AmountMinorUnits: r.AmountMinorUnits, Version: r.Version}, true, nil
}
func (tx *trialTx) AppendAdjustmentEvidence(ctx context.Context, e commercialapp.AdjustmentEvidence) error {
	return tx.appendAdjustmentEvidence(ctx, e)
}

func (tx *trialTx) LockAdjustmentCandidate(ctx context.Context, tenant, invoice domain.ID) (domain.AdjustmentCandidate, bool, error) {
	r, e := tx.commercial.LockAdjustmentCandidate(ctx, commercialsql.LockAdjustmentCandidateParams{InvoiceID: commercialID(invoice), TenantID: commercialID(tenant)})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.AdjustmentCandidate{}, false, nil
	}
	if e != nil {
		return domain.AdjustmentCandidate{}, false, e
	}
	taxHash, ok1 := hash32(r.TaxProfileHash)
	config, ok2 := hash32(r.ConfigurationHash)
	legal, ok3 := hash32(r.LegalReviewReferenceHash)
	if !ok1 || !ok2 || !ok3 || !r.SettledAt.Valid || !r.EffectiveFrom.Valid {
		return domain.AdjustmentCandidate{}, false, errors.New("commercial_adjustment_candidate_corrupt")
	}
	var until *time.Time
	if r.EffectiveUntil.Valid {
		v := r.EffectiveUntil.Time.UTC()
		until = &v
	}
	profile := domain.TaxProfile{ID: domain.ID(r.TaxProfileID.Bytes), Version: r.TaxProfileVersion, Country: r.Country, Currency: r.Currency, TaxCode: r.TaxCode, DisplayLabel: r.DisplayLabel, RateNumerator: r.RateNumerator, RateDenominator: r.RateDenominator, RoundingProfile: r.RoundingProfile, EffectiveFrom: r.EffectiveFrom.Time.UTC(), EffectiveUntil: until, ConfigurationHash: config, LegalReviewReferenceHash: legal}
	return domain.AdjustmentCandidate{InvoiceID: domain.ID(r.InvoiceID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), PaymentID: domain.ID(r.PaymentID.Bytes), InvoiceVersion: r.InvoiceVersion, PaymentVersion: r.PaymentVersion, InvoiceTaxableMinor: r.TaxableMinorUnits, InvoiceTaxMinor: r.TaxMinorUnits, InvoiceTotalMinor: r.TotalMinorUnits, PriorCreditMinor: r.PriorCreditMinorUnits, PaymentAmountMinor: r.AmountMinorUnits, PaymentProvider: r.Provider, EntitlementKind: r.EntitlementKind, SettledAt: r.SettledAt.Time.UTC(), TaxProfile: profile, TaxProfileHash: taxHash}, true, nil
}
func (tx *trialTx) InsertAdjustment(ctx context.Context, p domain.AdjustmentProposal, command domain.ID, key [32]byte, e commercialapp.AdjustmentEvidence) error {
	if err := tx.commercial.InsertFinancialChange(ctx, commercialsql.InsertFinancialChangeParams{ID: commercialID(p.ID), TenantID: commercialID(p.TenantID), InvoiceID: commercialID(p.InvoiceID), PaymentID: commercialID(p.PaymentID), Action: p.Action, ReasonCode: p.ReasonCode, AmountMinorUnits: p.AmountMinorUnits, NetMinorUnits: p.NetMinorUnits, TaxMinorUnits: p.TaxMinorUnits, TaxProfileID: commercialID(p.TaxProfileID), TaxProfileHash: p.TaxProfileHash[:], EntitlementDisposition: p.EntitlementDisposition, MakerUserID: commercialID(p.MakerUserID), MakerAuthorityHash: p.MakerAuthorityHash[:], ProposalReason: p.ProposalReason, ExpectedInvoiceVersion: p.ExpectedInvoiceVersion, ExpectedPaymentVersion: p.ExpectedPaymentVersion, RequestHash: p.RequestHash[:], At: commercialTime(p.CreatedAt), ExpiresAt: commercialTime(p.ExpiresAt)}); err != nil {
		return err
	}
	if err := tx.commercial.InsertAdjustmentCommand(ctx, commercialsql.InsertAdjustmentCommandParams{ID: commercialID(command), ActorUserID: commercialID(p.MakerUserID), TenantID: commercialID(p.TenantID), Operation: "PROPOSE_ADJUSTMENT", KeyHash: key[:], RequestHash: p.RequestHash[:], TargetID: commercialID(p.ID), ResponseState: "PENDING_APPROVAL", ResponseVersion: 1, ResponseStatus: 201, At: commercialTime(p.CreatedAt)}); err != nil {
		return err
	}
	return tx.appendAdjustmentEvidence(ctx, e)
}
func (tx *trialTx) LockCreditNoteNumber(ctx context.Context, year int, at time.Time) (int64, error) {
	if e := tx.commercial.EnsureFinancialNumberHead(ctx, commercialsql.EnsureFinancialNumberHeadParams{DocumentKind: "CREDIT_NOTE", CalendarYear: int32(year), At: commercialTime(at)}); e != nil {
		return 0, e
	}
	h, e := tx.commercial.LockFinancialNumberHead(ctx, commercialsql.LockFinancialNumberHeadParams{DocumentKind: "CREDIT_NOTE", CalendarYear: int32(year)})
	if e != nil {
		return 0, e
	}
	n, e := tx.commercial.AdvanceFinancialNumberHead(ctx, commercialsql.AdvanceFinancialNumberHeadParams{At: commercialTime(at), DocumentKind: "CREDIT_NOTE", CalendarYear: int32(year), ExpectedSequence: h.NextSequence, ExpectedVersion: h.Version})
	if e != nil {
		return 0, e
	}
	if n != 1 {
		return 0, commercialapp.ErrAdjustmentConflict
	}
	return h.NextSequence, nil
}
func (tx *trialTx) CommitAdjustmentDecision(ctx context.Context, d commercialapp.AdjustmentDecision, s commercialapp.AdjustmentStored, e commercialapp.AdjustmentEvidence) error {
	n, err := tx.commercial.DecideFinancialChange(ctx, commercialsql.DecideFinancialChangeParams{State: d.Decision, CheckerUserID: commercialID(d.CheckerUserID), CheckerAuthorityHash: d.AuthorityHash[:], DecisionReason: &d.Reason, At: commercialTime(d.At), ChangeID: commercialID(d.ChangeID), ExpectedVersion: d.ExpectedVersion})
	if err != nil {
		return err
	}
	if n != 1 {
		return commercialapp.ErrAdjustmentConflict
	}
	if err = tx.commercial.InsertFinancialChangeDecision(ctx, commercialsql.InsertFinancialChangeDecisionParams{ID: commercialID(d.DecisionID), ChangeID: commercialID(d.ChangeID), Decision: d.Decision, CheckerUserID: commercialID(d.CheckerUserID), ExpectedChangeVersion: d.ExpectedVersion, Reason: d.Reason, AuthorityHash: d.AuthorityHash[:], At: commercialTime(d.At)}); err != nil {
		return err
	}
	if d.Decision == "APPROVED" {
		if err = tx.commercial.InsertCreditNote(ctx, commercialsql.InsertCreditNoteParams{ID: commercialID(d.CreditNoteID), InvoiceID: commercialID(s.Proposal.InvoiceID), ChangeID: commercialID(d.ChangeID), CreditNumber: d.CreditNumber, State: d.CreditState, ReasonCode: s.Proposal.ReasonCode, NetMinorUnits: s.Proposal.NetMinorUnits, TaxMinorUnits: s.Proposal.TaxMinorUnits, TotalMinorUnits: s.Proposal.AmountMinorUnits, TaxProfileID: commercialID(s.Proposal.TaxProfileID), IssuedBy: commercialID(d.CheckerUserID), At: commercialTime(d.At), ContentHash: d.CreditHash[:]}); err != nil {
			return err
		}
		if s.Proposal.Action == "REQUEST_REFUND" {
			if err = tx.commercial.InsertRefundIntent(ctx, commercialsql.InsertRefundIntentParams{ID: commercialID(d.RefundID), ChangeID: commercialID(d.ChangeID), PaymentID: commercialID(s.Proposal.PaymentID), CreditNoteID: commercialID(d.CreditNoteID), Provider: "PESAPAL", AmountMinorUnits: s.Proposal.AmountMinorUnits, At: commercialTime(d.At)}); err != nil {
				return err
			}
			n, err = tx.commercial.MarkPaymentRefundPending(ctx, commercialsql.MarkPaymentRefundPendingParams{At: commercialTime(d.At), PaymentID: commercialID(s.Proposal.PaymentID), ExpectedVersion: s.Proposal.ExpectedPaymentVersion})
			if err != nil {
				return err
			}
			if n != 1 {
				return commercialapp.ErrAdjustmentConflict
			}
		}
	}
	operation := map[string]string{"APPROVED": "APPROVE_ADJUSTMENT", "REJECTED": "REJECT_ADJUSTMENT"}[d.Decision]
	if err = tx.commercial.InsertAdjustmentCommand(ctx, commercialsql.InsertAdjustmentCommandParams{ID: commercialID(d.CommandID), ActorUserID: commercialID(d.CheckerUserID), TenantID: commercialID(d.TenantID), Operation: operation, KeyHash: d.KeyHash[:], RequestHash: d.RequestHash[:], TargetID: commercialID(d.ChangeID), ResponseState: d.Decision, ResponseVersion: d.ExpectedVersion + 1, ResponseStatus: 200, At: commercialTime(d.At)}); err != nil {
		return err
	}
	return tx.appendAdjustmentEvidence(ctx, e)
}
func (tx *trialTx) appendAdjustmentEvidence(ctx context.Context, e commercialapp.AdjustmentEvidence) error {
	return tx.evidence.AppendCommercialAdjustment(ctx, controlapp.CommercialAdjustmentEvidence{AuditID: controlapp.ID(e.AuditID), OutboxID: controlapp.ID(e.OutboxID), CorrelationID: controlapp.ID(e.CorrelationID), ActorID: controlapp.ID(e.ActorID), TenantID: controlapp.ID(e.TenantID), InvoiceID: controlapp.ID(e.InvoiceID), PaymentID: controlapp.ID(e.PaymentID), ChangeID: controlapp.ID(e.ChangeID), EventType: e.EventType, State: e.State, Action: e.Action, ReasonCode: e.ReasonCode, EntitlementDisposition: e.EntitlementDisposition, Version: e.Version, AmountMinorUnits: e.AmountMinorUnits, OccurredAt: e.OccurredAt})
}
func hash32(v []byte) ([32]byte, bool) {
	var h [32]byte
	if len(v) != 32 {
		return h, false
	}
	copy(h[:], v)
	return h, true
}
func mustHashes(a, b, c []byte) ([32]byte, [32]byte, [32]byte) {
	x, _ := hash32(a)
	y, _ := hash32(b)
	z, _ := hash32(c)
	return x, y, z
}
