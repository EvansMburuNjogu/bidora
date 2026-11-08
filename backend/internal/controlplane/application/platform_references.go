package application

import (
	p "bidora/backend/internal/platformadmin/application"
	policy "bidora/backend/internal/platformadmin/domain"
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type PlatformReference struct {
	ID    ID
	Label string
}
type PlatformReferencePage struct {
	Items   []PlatformReference
	HasMore bool
}
type platformReferenceTx interface {
	LookupReferences(context.Context, string, string) ([]PlatformReference, error)
	AppendReferenceRead(context.Context, ID, PlatformAuditSubject, time.Time) error
}

func (s *PlatformOverviewService) Lookup(ctx context.Context, subject PlatformAuditSubject, kind, search string) (out PlatformReferencePage, err error) {
	if ctx == nil || !validAuditSubject(subject) || (kind != "organizations" && kind != "support-assignees") || !utf8.ValidString(search) || len(search) > 100 || strings.TrimSpace(search) != search || strings.IndexFunc(search, func(r rune) bool { return r < 32 }) >= 0 {
		return out, ErrPlatformAuditInvalid
	}
	at := s.clock.Now().UTC()
	id, e := s.ids.NewID()
	if e != nil || id.IsZero() || at.IsZero() {
		return out, ErrPlatformAuditUnavailable
	}
	err = s.store.WithinOverview(ctx, func(c context.Context, tx PlatformOverviewTx) error {
		a, e := tx.ResolvePlatformAuthority(c, p.AuthorityQuery{UserID: p.ID(subject.UserID), SessionID: p.ID(subject.SessionID), ObservedAt: at})
		if errors.Is(e, p.ErrAuthorityNotFound) {
			return ErrPlatformAuditForbidden
		}
		if e != nil {
			return e
		}
		if a.UserID != p.ID(subject.UserID) || a.ContextGeneration != subject.ContextGeneration || !policy.AdministratorAssurance(a.AssuranceLevel, a.AssuranceMethod) || a.AssuredAt.IsZero() || a.AssuredAt.After(at) || a.AssuredAt.Before(at.Add(-15*time.Minute)) {
			return ErrPlatformAuditForbidden
		}
		allowed := a.Grants("platform.tenant.read") || a.Grants("platform.subscription.read") || a.Grants("platform.support.case.read")
		if kind == "support-assignees" {
			allowed = a.Grants("platform.support.case.manage")
		}
		if !allowed {
			return ErrPlatformAuditForbidden
		}
		references, ok := tx.(platformReferenceTx)
		if !ok {
			return ErrPlatformAuditUnavailable
		}
		rows, e := references.LookupReferences(c, kind, search)
		if e != nil {
			return e
		}
		out.HasMore = len(rows) > 25
		if out.HasMore {
			rows = rows[:25]
		}
		out.Items = rows
		return references.AppendReferenceRead(c, id, subject, at)
	})
	if err != nil {
		out = PlatformReferencePage{}
	}
	return out, err
}
