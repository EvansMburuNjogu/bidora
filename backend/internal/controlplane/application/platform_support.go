package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	platformapp "bidora/backend/internal/platformadmin/application"
	platformpolicy "bidora/backend/internal/platformadmin/domain"
)

var ErrSupportInvalid = errors.New("platform_support_invalid")
var ErrSupportForbidden = errors.New("platform_support_forbidden")
var ErrSupportNotFound = errors.New("platform_support_not_found")
var ErrSupportConflict = errors.New("platform_support_conflict")
var ErrSupportUnavailable = errors.New("platform_support_unavailable")

type SupportCase struct {
	ID, TenantID, AssignedTo, CreatedBy ID
	Title, Description, State           string
	Version                             int64
	CreatedAt, UpdatedAt                time.Time
}
type SupportMessage struct {
	ID, ActorID ID
	Body        string
	CreatedAt   time.Time
}
type SupportDetail struct {
	Case     SupportCase
	Messages []SupportMessage
}
type SupportPage struct {
	Cases []SupportCase
	Next  *PlatformAuditCursor
}
type SupportReceipt struct {
	CaseID  ID
	Version int64
	Replay  bool
}
type SupportCommand struct {
	Subject                                     PlatformAuditSubject
	Operation                                   string // CREATE, ASSIGN, REPLY, RESOLVE, REOPEN
	CaseID, TenantID, Assignee                  ID
	Title, Description, Message, IdempotencyKey string
	ExpectedVersion                             int64
}
type SupportCommandRecord struct {
	RequestHash [32]byte
	CaseID      ID
	Version     int64
}
type SupportTx interface {
	ResolvePlatformAuthority(context.Context, platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error)
	ListCases(context.Context, int32, *PlatformAuditCursor) ([]SupportCase, error)
	ReadCase(context.Context, ID, bool) (SupportCase, bool, error)
	ListMessages(context.Context, ID) ([]SupportMessage, error)
	TenantExists(context.Context, ID) (bool, error)
	AssigneeEligible(context.Context, ID) (bool, error)
	MessageCount(context.Context, ID) (int64, error)
	FindCommand(context.Context, ID, string, [32]byte) (SupportCommandRecord, bool, error)
	InsertCase(context.Context, SupportCase) error
	UpdateCase(context.Context, SupportCase, int64) (bool, error)
	InsertMessage(context.Context, ID, SupportMessage) error
	AppendEvent(context.Context, ID, ID, ID, string, int64, time.Time) error
	AppendCommand(context.Context, ID, string, [32]byte, [32]byte, ID, int64, time.Time) error
	AppendAudit(context.Context, ID, ID, ID, ID, string, time.Time) error
}
type SupportUnitOfWork interface {
	WithinSupport(context.Context, func(context.Context, SupportTx) error) error
}
type SupportService struct {
	uow   SupportUnitOfWork
	ids   PlatformAuditIDs
	clock PlatformAuditClock
}

func NewSupportService(uow SupportUnitOfWork, ids PlatformAuditIDs, clock PlatformAuditClock) (*SupportService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrSupportUnavailable
	}
	return &SupportService{uow, ids, clock}, nil
}
func supportText(v string, min, max int) bool {
	return len(v) >= min && len(v) <= max && strings.TrimSpace(v) == v && strings.IndexFunc(v, func(r rune) bool { return r < 32 && r != '\n' }) < 0
}
func (s *SupportService) authorize(ctx context.Context, tx SupportTx, subject PlatformAuditSubject, at time.Time, capability string) error {
	facts, err := tx.ResolvePlatformAuthority(ctx, platformapp.AuthorityQuery{UserID: platformapp.ID(subject.UserID), SessionID: platformapp.ID(subject.SessionID), ObservedAt: at})
	if errors.Is(err, platformapp.ErrAuthorityNotFound) {
		return ErrSupportForbidden
	}
	if err != nil {
		return err
	}
	if !facts.Grants(capability) || facts.UserID != platformapp.ID(subject.UserID) || facts.ContextGeneration != subject.ContextGeneration || !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.IsZero() || facts.AssuredAt.After(at) || facts.AssuredAt.Before(at.Add(-15*time.Minute)) {
		return ErrSupportForbidden
	}
	return nil
}
func supportError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrSupportInvalid), errors.Is(err, ErrSupportForbidden), errors.Is(err, ErrSupportConflict), errors.Is(err, ErrSupportNotFound):
		return err
	default:
		return ErrSupportUnavailable
	}
}
func (s *SupportService) newID() (ID, error) {
	id, err := s.ids.NewID()
	if err != nil || id.IsZero() {
		return ID{}, ErrSupportUnavailable
	}
	return id, nil
}
func (s *SupportService) List(ctx context.Context, subject PlatformAuditSubject, limit int32, cursor *PlatformAuditCursor) (out SupportPage, err error) {
	if s == nil || ctx == nil || !validAuditSubject(subject) || limit < 1 || limit > 50 || cursor != nil && (cursor.ID.IsZero() || cursor.OccurredAt.IsZero()) {
		return out, ErrSupportInvalid
	}
	at := s.clock.Now().UTC()
	audit, e := s.newID()
	if at.IsZero() || e != nil {
		return out, ErrSupportUnavailable
	}
	err = s.uow.WithinSupport(ctx, func(c context.Context, tx SupportTx) error {
		if x := s.authorize(c, tx, subject, at, "platform.support.case.read"); x != nil {
			return x
		}
		rows, x := tx.ListCases(c, limit+1, cursor)
		if x != nil {
			return x
		}
		if len(rows) > int(limit) {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			out.Next = &PlatformAuditCursor{OccurredAt: last.UpdatedAt, ID: last.ID}
		}
		out.Cases = rows
		return tx.AppendAudit(c, audit, subject.UserID, subject.CorrelationID, audit, "PlatformSupportCasesRead", at)
	})
	if err != nil {
		return SupportPage{}, supportError(err)
	}
	return out, nil
}
func (s *SupportService) Detail(ctx context.Context, subject PlatformAuditSubject, id ID) (out SupportDetail, err error) {
	if s == nil || ctx == nil || !validAuditSubject(subject) || id.IsZero() {
		return out, ErrSupportInvalid
	}
	at := s.clock.Now().UTC()
	audit, e := s.newID()
	if at.IsZero() || e != nil {
		return out, ErrSupportUnavailable
	}
	err = s.uow.WithinSupport(ctx, func(c context.Context, tx SupportTx) error {
		if x := s.authorize(c, tx, subject, at, "platform.support.case.read"); x != nil {
			return x
		}
		row, ok, x := tx.ReadCase(c, id, false)
		if x != nil {
			return x
		}
		if !ok {
			return ErrSupportNotFound
		}
		out.Case = row
		out.Messages, x = tx.ListMessages(c, id)
		if x != nil {
			return x
		}
		if len(out.Messages) > 100 {
			return ErrSupportUnavailable
		}
		return tx.AppendAudit(c, audit, subject.UserID, subject.CorrelationID, id, "PlatformSupportCaseRead", at)
	})
	if err != nil {
		return SupportDetail{}, supportError(err)
	}
	return out, nil
}
func validSupportCommand(in SupportCommand) bool {
	if !validAuditSubject(in.Subject) || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 128 {
		return false
	}
	switch in.Operation {
	case "CREATE":
		return in.CaseID.IsZero() && !in.TenantID.IsZero() && in.Assignee.IsZero() && in.ExpectedVersion == 0 && in.Message == "" && supportText(in.Title, 3, 120) && supportText(in.Description, 3, 2000)
	case "ASSIGN":
		return !in.CaseID.IsZero() && !in.Assignee.IsZero() && in.TenantID.IsZero() && in.ExpectedVersion > 0 && in.Title == "" && in.Description == "" && in.Message == ""
	case "REPLY":
		return !in.CaseID.IsZero() && in.Assignee.IsZero() && in.TenantID.IsZero() && in.ExpectedVersion > 0 && in.Title == "" && in.Description == "" && supportText(in.Message, 1, 2000)
	case "RESOLVE", "REOPEN":
		return !in.CaseID.IsZero() && in.Assignee.IsZero() && in.TenantID.IsZero() && in.ExpectedVersion > 0 && in.Title == "" && in.Description == "" && in.Message == ""
	}
	return false
}
func (s *SupportService) Command(ctx context.Context, in SupportCommand) (out SupportReceipt, err error) {
	if s == nil || ctx == nil || !validSupportCommand(in) {
		return out, ErrSupportInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return out, ErrSupportUnavailable
	}
	body, _ := json.Marshal(struct {
		Actor, Case, Tenant, Assignee          ID
		Operation, Title, Description, Message string
		Generation, Version                    int64
	}{in.Subject.UserID, in.CaseID, in.TenantID, in.Assignee, in.Operation, in.Title, in.Description, in.Message, in.Subject.ContextGeneration, in.ExpectedVersion})
	keyHash := sha256.Sum256([]byte("platform-support:" + in.Operation + ":" + in.IdempotencyKey))
	requestHash := sha256.Sum256(append([]byte("platform-support:v1:"), body...))
	err = s.uow.WithinSupport(ctx, func(c context.Context, tx SupportTx) error {
		if x := s.authorize(c, tx, in.Subject, at, "platform.support.case.manage"); x != nil {
			return x
		}
		record, found, x := tx.FindCommand(c, in.Subject.UserID, in.Operation, keyHash)
		if x != nil {
			return x
		}
		if found {
			if record.RequestHash != requestHash {
				return ErrSupportConflict
			}
			out = SupportReceipt{CaseID: record.CaseID, Version: record.Version, Replay: true}
			return nil
		}
		var row SupportCase
		if in.Operation == "CREATE" {
			exists, x := tx.TenantExists(c, in.TenantID)
			if x != nil {
				return x
			}
			if !exists {
				return ErrSupportNotFound
			}
			id, x := s.newID()
			if x != nil {
				return x
			}
			row = SupportCase{ID: id, TenantID: in.TenantID, CreatedBy: in.Subject.UserID, Title: in.Title, Description: in.Description, State: "OPEN", Version: 1, CreatedAt: at, UpdatedAt: at}
			if x = tx.InsertCase(c, row); x != nil {
				return x
			}
		} else {
			var ok bool
			row, ok, x = tx.ReadCase(c, in.CaseID, true)
			if x != nil {
				return x
			}
			if !ok {
				return ErrSupportNotFound
			}
			if row.Version != in.ExpectedVersion {
				return ErrSupportConflict
			}
			switch in.Operation {
			case "ASSIGN":
				eligible, e := tx.AssigneeEligible(c, in.Assignee)
				if e != nil {
					return e
				}
				if !eligible {
					return ErrSupportInvalid
				}
				if row.State != "OPEN" || row.AssignedTo == in.Assignee {
					return ErrSupportConflict
				}
				row.AssignedTo = in.Assignee
			case "REPLY":
				if row.State != "OPEN" {
					return ErrSupportConflict
				}
				count, e := tx.MessageCount(c, row.ID)
				if e != nil {
					return e
				}
				if count >= 100 {
					return ErrSupportConflict
				}
			case "RESOLVE":
				if row.State != "OPEN" {
					return ErrSupportConflict
				}
				row.State = "RESOLVED"
			case "REOPEN":
				if row.State != "RESOLVED" {
					return ErrSupportConflict
				}
				row.State = "OPEN"
			}
			previous := row.Version
			row.Version++
			row.UpdatedAt = at
			updated, e := tx.UpdateCase(c, row, previous)
			if e != nil {
				return e
			}
			if !updated {
				return ErrSupportConflict
			}
		}
		if in.Operation == "REPLY" {
			messageID, e := s.newID()
			if e != nil {
				return e
			}
			if e = tx.InsertMessage(c, row.ID, SupportMessage{ID: messageID, ActorID: in.Subject.UserID, Body: in.Message, CreatedAt: at}); e != nil {
				return e
			}
		}
		eventID, e := s.newID()
		if e != nil {
			return e
		}
		if e = tx.AppendEvent(c, eventID, row.ID, in.Subject.UserID, in.Operation, row.Version, at); e != nil {
			return e
		}
		if e = tx.AppendCommand(c, in.Subject.UserID, in.Operation, keyHash, requestHash, row.ID, row.Version, at); e != nil {
			return e
		}
		auditID, e := s.newID()
		if e != nil {
			return e
		}
		if e = tx.AppendAudit(c, auditID, in.Subject.UserID, in.Subject.CorrelationID, row.ID, "PlatformSupportCase"+strings.Title(strings.ToLower(in.Operation)), at); e != nil {
			return e
		}
		out = SupportReceipt{CaseID: row.ID, Version: row.Version}
		return nil
	})
	if err != nil {
		return SupportReceipt{}, supportError(err)
	}
	return out, nil
}
