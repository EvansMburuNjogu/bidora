package persistence

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	q "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"encoding/json"
	"errors"
	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *RegionalPolicyStore) CurrentPolicy(ctx context.Context) (out app.OperationalPolicy, err error) {
	err = s.WithinRegionalPolicy(ctx, false, func(c context.Context, t app.RegionalPolicyTx) error {
		var e error
		out, e = t.Policy(c, false)
		if e == nil && !out.Valid() {
			return app.ErrInformationUnavailable
		}
		return e
	})
	return out, err
}
func (s *RegionalPolicyStore) UploadLimit(ctx context.Context) (int64, error) {
	p, e := s.CurrentPolicy(ctx)
	if e != nil || !p.UploadsEnabled || p.UnderMaintenance(time.Now().UTC()) {
		return 0, app.ErrInformationUnavailable
	}
	return p.MaxDocumentBytes, nil
}
func (s *RegionalPolicyStore) Gate(ctx context.Context, path string) error {
	if !strings.HasPrefix(path, "/v1/tenderpreneur/") && !strings.HasPrefix(path, "/v1/shared/invitations") {
		return nil
	}
	p, e := s.CurrentPolicy(ctx)
	if e != nil {
		return e
	}
	if p.UnderMaintenance(time.Now().UTC()) {
		return app.OperationalUnavailable{Message: p.MaintenanceMessage}
	}
	if strings.Contains(path, "/uploads") && !p.UploadsEnabled || path == "/v1/tenderpreneur/organizations" && !p.RegistrationEnabled {
		return app.ErrInformationUnavailable
	}
	return nil
}

// Observation captures authenticated Platform activity only using trusted-peer
// network resolution. Retention here masks reads; it does
// not claim physical deletion or create destructive automation/legal holds.
func (s *RegionalPolicyStore) ObserveRequest(ctx context.Context, actor, correlation domain.ID, ip netip.Addr, browser string, at time.Time) error {
	if actor.IsZero() || correlation.IsZero() || !ip.IsValid() || len(browser) > 512 || !utf8.ValidString(browser) || strings.ContainsAny(browser, "\r\n\x00") {
		return app.ErrInformationInvalid
	}
	r, ok := s.runner.(transactionRunner)
	if !ok {
		return app.ErrInformationUnavailable
	}
	return r.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		return q.New(db).InsertRestrictedRequestMetadata(c, q.InsertRestrictedRequestMetadataParams{CorrelationID: uuid(correlation), ActorID: uuid(actor), IpAddress: ip, Browser: browser, At: ts(at)})
	})
}

type RequestMetadata = app.RequestMetadata

func (s *RegionalPolicyStore) RequestMetadata(ctx context.Context, subject app.RegionalSubject, audit domain.ID, at time.Time) (out RequestMetadata, err error) {
	writer, ok := s.runner.(transactionRunner)
	if !ok {
		return out, app.ErrInformationUnavailable
	}
	err = writer.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		t, e := NewRegionalPolicyTx(db)
		if e != nil {
			return e
		}
		if e = t.AuthorizeRegional(c, subject, "METADATA", false, at); e != nil {
			return e
		}
		p, e := t.Policy(c, false)
		if e != nil || !p.Valid() {
			return app.ErrInformationUnavailable
		}
		v, e := q.New(db).ReadRestrictedRequestMetadata(c, q.ReadRestrictedRequestMetadataParams{AuditID: uuid(audit), Cutoff: ts(at.Add(-time.Duration(p.MetadataRetentionDays) * 24 * time.Hour))})
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrNotFound
		}
		if e != nil {
			return e
		}
		id, e := googleuuid.NewRandom()
		if e != nil {
			return e
		}
		summary, _ := json.Marshal(map[string]any{"source_audit_id": googleuuid.UUID(audit).String(), "policy_version": p.Version})
		if e = q.New(db).AppendRegionalPolicyAudit(c, q.AppendRegionalPolicyAuditParams{ID: uuid(domain.ID(id)), ActorID: uuid(subject.ActorID), Action: "RestrictedRequestMetadataRead", TargetID: uuid(audit), At: ts(at), CorrelationID: uuid(subject.CorrelationID), Summary: summary}); e != nil {
			return e
		}
		out = RequestMetadata{IP: v.IpAddress.String(), Browser: v.Browser, ObservedAt: v.ObservedAt.Time.UTC()}
		return nil
	})
	return out, err
}
