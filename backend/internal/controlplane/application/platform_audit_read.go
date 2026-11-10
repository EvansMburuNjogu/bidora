package application

import (
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"errors"
	"strings"
	"time"

	"bidora/backend/internal/identity/securitypolicy"
	platformapp "bidora/backend/internal/platformadmin/application"
)

var ErrPlatformAuditInvalid = errors.New("platform_audit_read_invalid")
var ErrPlatformAuditForbidden = errors.New("platform_audit_read_forbidden")
var ErrPlatformAuditNotFound = errors.New("platform_audit_read_not_found")
var ErrPlatformAuditStepUpRequired = errors.New("platform_audit_step_up_required")
var ErrPlatformAuditUnavailable = errors.New("platform_audit_read_unavailable")

type PlatformAuditSubject struct {
	UserID, SessionID, CorrelationID ID
	ContextGeneration                int64
	MetadataAccess                   bool // Internal read-evidence flag, never populated from HTTP.
}
type PlatformAuditCursor struct {
	OccurredAt time.Time
	ID         ID
}
type PlatformAuditFilter struct {
	IncludeMetadata              bool // Set only after server-side authorization.
	Search                       string
	Administrator                ID
	Action, Module, Result, Sort string
	From, Before                 time.Time // UTC inclusive start, exclusive end; zero means unbounded
}
type PlatformAuditChange struct {
	Field     string  `json:"field"`
	Operation string  `json:"operation"`
	Before    *string `json:"before"`
	After     *string `json:"after"`
}
type PlatformAuditAction struct {
	AdministratorName, AffectedRecordName string
	IPAddress, Browser                    string
	Changes                               []PlatformAuditChange
	ID, Administrator, AffectedRecord     ID
	Action, Module, Result                string
	OccurredAt                            time.Time
}
type PlatformAuditPage struct {
	Actions []PlatformAuditAction
	Next    *PlatformAuditCursor
}
type PlatformAuditTx interface {
	ResolvePlatformAuthority(context.Context, platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error)
	ListPlatformAuditActions(context.Context, int32, PlatformAuditFilter, *PlatformAuditCursor) ([]PlatformAuditAction, error)
	GetPlatformAuditAction(context.Context, ID, bool) (PlatformAuditAction, bool, error)
	AppendPlatformAuditRead(context.Context, ID, PlatformAuditSubject, time.Time) error
	AppendPlatformAuditExport(context.Context, ID, PlatformAuditSubject, time.Time) error
}
type PlatformAuditUnitOfWork interface {
	WithinPlatformAudit(context.Context, func(context.Context, PlatformAuditTx) error) error
}
type PlatformAuditIDs interface{ NewID() (ID, error) }
type PlatformAuditClock interface{ Now() time.Time }
type PlatformAuditService struct {
	uow   PlatformAuditUnitOfWork
	ids   PlatformAuditIDs
	clock PlatformAuditClock
}

func NewPlatformAuditService(uow PlatformAuditUnitOfWork, ids PlatformAuditIDs, clock PlatformAuditClock) (*PlatformAuditService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("platform_audit_service_invalid")
	}
	return &PlatformAuditService{uow, ids, clock}, nil
}

var platformAuditActions = map[string]bool{
	"PlatformTenantSuspended": true, "PlatformTenantReactivated": true,
	"PlatformInformationUpdated": true, "PlatformSessionPolicyUpdated": true,
	"CountryConfigurationUpdated": true, "OperationalPolicyUpdated": true, "RestrictedRequestMetadataRead": true,
	"PlatformGoogleConfigurationUpdated": true, "PlatformGoogleConnectionTested": true, "NotificationTemplateUpdated": true, "NotificationTemplateTested": true,
	"UserLoggedIn": true, "EmailLoginCodeVerified": true, "MFALoginCompleted": true,
	"PlatformOwnerBootstrapped": true, "PlatformAdministratorSessionsRevoked": true,
	"PlatformAdministratorInvitationIssued": true, "PlatformAdministratorInvitationCancelled": true, "PlatformAdministratorInvitationAccepted": true, "PlatformOwnerInvitationActivated": true, "PlatformOwnerInvitationRejected": true,
	"PlatformRoleAssignmentChanged": true, "PlatformRoleRevocationChanged": true, "PlatformAdministratorDisabled": true, "PlatformAdministratorReactivated": true, "PlatformAdministratorDeleted": true,
	"PlatformIntegrationVersionCreated": true, "PlatformIntegrationVersionTested": true, "PlatformIntegrationActivationProposed": true, "PlatformControlAuditExport": true,
	"PlatformPlansRead": true, "PlatformPlanDraftCreated": true, "PlatformPlanPublished": true,
	"PlatformSubscriptionRead": true, "PlatformSubscriptionCancellationScheduled": true, "PlatformSubscriptionCancellationWithdrawn": true, "PlatformSubscriptionInventoryRead": true,
	"PlatformSupportCasesRead": true, "PlatformSupportCaseRead": true, "PlatformSupportCaseCreate": true, "PlatformSupportCaseAssign": true, "PlatformSupportCaseReply": true, "PlatformSupportCaseResolve": true, "PlatformSupportCaseReopen": true,
}

func AllowedPlatformAuditAction(v string) bool { return platformAuditActions[v] }
func ValidPlatformAuditFilter(f PlatformAuditFilter) bool {
	if len(f.Search) > 80 || strings.TrimSpace(f.Search) != f.Search || strings.IndexFunc(f.Search, func(r rune) bool { return r < 32 || r > 126 }) >= 0 ||
		f.Action != "" && !AllowedPlatformAuditAction(f.Action) || f.Module != "" && f.Module != "ADMINISTRATION" && f.Module != "INTEGRATIONS" ||
		f.Result != "" && f.Result != "SUCCESSFUL" && f.Result != "FAILED" && f.Result != "PENDING" ||
		f.Sort != "ASC" && f.Sort != "DESC" || !f.From.IsZero() && !f.Before.IsZero() && !f.Before.After(f.From) {
		return false
	}
	return true
}
func validAuditSubject(s PlatformAuditSubject) bool {
	return !s.UserID.IsZero() && !s.SessionID.IsZero() && !s.CorrelationID.IsZero() && s.ContextGeneration > 0
}
func (s *PlatformAuditService) List(ctx context.Context, subject PlatformAuditSubject, filter PlatformAuditFilter, limit int32, cursor *PlatformAuditCursor) (out PlatformAuditPage, err error) {
	if ctx == nil || s == nil || !validAuditSubject(subject) || !ValidPlatformAuditFilter(filter) || limit < 1 || limit > 100 || cursor != nil && (cursor.ID.IsZero() || cursor.OccurredAt.IsZero()) {
		return out, ErrPlatformAuditInvalid
	}
	err = s.read(ctx, subject, false, func(c context.Context, tx PlatformAuditTx, metadata bool) error {
		filter.IncludeMetadata = metadata
		rows, x := tx.ListPlatformAuditActions(c, limit+1, filter, cursor)
		if x != nil {
			return x
		}
		if len(rows) > int(limit) {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			out.Next = &PlatformAuditCursor{OccurredAt: last.OccurredAt, ID: last.ID}
		}
		out.Actions = rows
		return nil
	})
	if err != nil {
		return PlatformAuditPage{}, err
	}
	return out, nil
}

// Export commits a bounded UTC window with distinct export evidence, never a partial archive.
func (s *PlatformAuditService) Export(ctx context.Context, subject PlatformAuditSubject, filter PlatformAuditFilter) (out PlatformAuditPage, err error) {
	if ctx == nil || s == nil || !validAuditSubject(subject) || !ValidPlatformAuditFilter(filter) || filter.From.IsZero() || filter.Before.IsZero() || filter.Before.Sub(filter.From) > 31*24*time.Hour {
		return out, ErrPlatformAuditInvalid
	}
	err = s.read(ctx, subject, true, func(c context.Context, tx PlatformAuditTx, _ bool) error {
		filter.IncludeMetadata = false // Restricted network data is not part of CSV export.
		rows, x := tx.ListPlatformAuditActions(c, 501, filter, nil)
		if x != nil {
			return x
		}
		if len(rows) > 500 {
			return ErrPlatformAuditInvalid
		} // Never return a partial export.
		out.Actions = rows
		return nil
	})
	if err != nil {
		return PlatformAuditPage{}, err
	}
	return out, nil
}
func (s *PlatformAuditService) Detail(ctx context.Context, subject PlatformAuditSubject, id ID) (out PlatformAuditAction, err error) {
	if ctx == nil || s == nil || !validAuditSubject(subject) || id.IsZero() {
		return out, ErrPlatformAuditInvalid
	}
	err = s.read(ctx, subject, false, func(c context.Context, tx PlatformAuditTx, metadata bool) error {
		var found bool
		var x error
		out, found, x = tx.GetPlatformAuditAction(c, id, metadata)
		if x != nil {
			return x
		}
		if !found {
			return ErrPlatformAuditNotFound
		}
		return nil
	})
	if err != nil {
		return PlatformAuditAction{}, err
	}
	return out, nil
}
func (s *PlatformAuditService) read(ctx context.Context, subject PlatformAuditSubject, export bool, load func(context.Context, PlatformAuditTx, bool) error) error {
	at := s.clock.Now().UTC()
	auditID, e := s.ids.NewID()
	if at.IsZero() || e != nil || auditID.IsZero() {
		return ErrPlatformAuditUnavailable
	}
	err := s.uow.WithinPlatformAudit(ctx, func(c context.Context, tx PlatformAuditTx) error {
		facts, x := tx.ResolvePlatformAuthority(c, platformapp.AuthorityQuery{UserID: platformapp.ID(subject.UserID), SessionID: platformapp.ID(subject.SessionID), ObservedAt: at})
		if errors.Is(x, platformapp.ErrAuthorityNotFound) {
			return ErrPlatformAuditForbidden
		}
		if x != nil {
			return x
		}
		if !facts.Grants("platform.audit.read") || export && !facts.Grants("platform.audit.export") || facts.UserID != platformapp.ID(subject.UserID) || facts.ContextGeneration != subject.ContextGeneration || !platformpolicy.CurrentAdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod, facts.AssuredAt, at) {
			return ErrPlatformAuditForbidden
		}
		if export && !securitypolicy.Recent(ctx, facts.AssuredAt, at) {
			return ErrPlatformAuditStepUpRequired
		}
		subject.MetadataAccess = !export && facts.Grants("platform.security.event.read") && securitypolicy.Recent(ctx, facts.AssuredAt, at)
		if x = load(c, tx, subject.MetadataAccess); x != nil {
			return x
		}
		if export {
			return tx.AppendPlatformAuditExport(c, auditID, subject, at)
		}
		return tx.AppendPlatformAuditRead(c, auditID, subject, at)
	})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrPlatformAuditForbidden), errors.Is(err, ErrPlatformAuditNotFound), errors.Is(err, ErrPlatformAuditInvalid), errors.Is(err, ErrPlatformAuditStepUpRequired):
		return err
	default:
		return ErrPlatformAuditUnavailable
	}
}
