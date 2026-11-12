package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	q "bidora/backend/internal/platformsettings/persistence/sqlc"
	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (t *RegionalPolicyTx) ListOrganizationReferences(ctx context.Context, kind string, activeOnly bool) ([]app.OrganizationReference, error) {
	rows, err := t.q.ListOrganizationReferences(ctx, q.ListOrganizationReferencesParams{Kind: kind, ActiveOnly: activeOnly})
	if err != nil {
		return nil, err
	}
	out := make([]app.OrganizationReference, 0, len(rows))
	for _, r := range rows {
		out = append(out, app.OrganizationReference{Kind: r.Kind, Code: r.Code, Label: r.Label, Enabled: r.Enabled, Version: r.Version, UpdatedAt: r.UpdatedAt.Time.UTC()})
	}
	return out, nil
}
func (t *RegionalPolicyTx) LockOrganizationReference(ctx context.Context, kind, code string) (app.OrganizationReference, error) {
	r, err := t.q.LockOrganizationReference(ctx, q.LockOrganizationReferenceParams{Kind: kind, Code: code})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.OrganizationReference{}, app.ErrInformationConflict
	}
	if err != nil {
		return app.OrganizationReference{}, err
	}
	return app.OrganizationReference{Kind: r.Kind, Code: r.Code, Label: r.Label, Enabled: r.Enabled, Version: r.Version, UpdatedAt: r.UpdatedAt.Time.UTC()}, nil
}
func (t *RegionalPolicyTx) SaveOrganizationReference(ctx context.Context, v app.OrganizationReference, s app.RegionalSubject, at time.Time) error {
	var count int64
	var err error
	if v.Version == 0 {
		count, err = t.q.CreateOrganizationReference(ctx, q.CreateOrganizationReferenceParams{Kind: v.Kind, Code: v.Code, Label: v.Label, Enabled: v.Enabled, ActorID: uuid(s.ActorID), At: ts(at)})
	} else {
		count, err = t.q.UpdateOrganizationReference(ctx, q.UpdateOrganizationReferenceParams{Kind: v.Kind, Code: v.Code, Label: v.Label, Enabled: v.Enabled, ExpectedVersion: v.Version, ActorID: uuid(s.ActorID), At: ts(at)})
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return app.ErrInformationConflict
	}
	if err != nil {
		return err
	}
	if count != 1 {
		return app.ErrInformationConflict
	}
	return nil
}
func (t *RegionalPolicyTx) OrganizationReferenceCommand(ctx context.Context, s app.RegionalSubject, key [32]byte) (app.OrganizationReferenceReceipt, bool, error) {
	r, err := t.q.ReadOrganizationReferenceCommand(ctx, q.ReadOrganizationReferenceCommandParams{ActorID: uuid(s.ActorID), KeyHash: key[:]})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.OrganizationReferenceReceipt{}, false, nil
	}
	if err != nil {
		return app.OrganizationReferenceReceipt{}, false, err
	}
	return app.OrganizationReferenceReceipt{Hash: hash32(r.RequestHash), Kind: r.Kind, Code: r.Code, Version: r.ResultVersion}, true, nil
}
func (t *RegionalPolicyTx) OrganizationReferenceEvidence(ctx context.Context, s app.RegionalSubject, v app.OrganizationReference, key, hash [32]byte, audit, event domain.ID, at time.Time) error {
	if s.CorrelationID.IsZero() {
		return app.ErrInformationInvalid
	}
	if err := t.q.InsertOrganizationReferenceCommand(ctx, q.InsertOrganizationReferenceCommandParams{ActorID: uuid(s.ActorID), KeyHash: key[:], RequestHash: hash[:], Kind: v.Kind, Code: v.Code, ResultVersion: v.Version, At: ts(at)}); err != nil {
		return err
	}
	target := domain.ID(googleuuid.NewSHA1(googleuuid.NameSpaceURL, []byte("bidora:organization-reference:"+v.Kind+":"+v.Code)))
	action := "OrganizationBusinessTypeUpdated"
	if v.Kind == "INDUSTRY" {
		action = "OrganizationIndustryUpdated"
	}
	summary, _ := json.Marshal(map[string]any{"kind": v.Kind, "target": v.Code, "version": v.Version, "changes": s.Changes})
	if err := t.q.AppendRegionalPolicyAudit(ctx, q.AppendRegionalPolicyAuditParams{ID: uuid(audit), ActorID: uuid(s.ActorID), Action: action, TargetID: uuid(target), At: ts(at), CorrelationID: uuid(s.CorrelationID), Summary: summary}); err != nil {
		return err
	}
	return t.q.AppendRegionalPolicyOutbox(ctx, q.AppendRegionalPolicyOutboxParams{ID: uuid(event), ActorID: uuid(s.ActorID), Action: action, TargetID: uuid(target), Version: v.Version, At: ts(at), Summary: summary})
}

func (t *RegionalPolicyTx) FenceActiveOrganizationReference(ctx context.Context, kind, code string) error {
	_, err := t.q.FenceActiveOrganizationReference(ctx, q.FenceActiveOrganizationReferenceParams{Kind: kind, Code: code})
	return err
}
