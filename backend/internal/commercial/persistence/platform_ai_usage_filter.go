package persistence

import (
	app "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	q "bidora/backend/internal/commercial/persistence/sqlc"
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgtype"
)

func (t *trialTx) ListFilteredAIUsage(c context.Context, limit int32, cursor *app.CommercialListCursor, filter app.PlatformAIUsageFilter) ([]app.PlatformAIUsageItem, error) {
	at, id := listCursor(cursor)
	var tenant pgtype.UUID
	if !filter.TenantID.IsZero() {
		tenant = pgtype.UUID{Bytes: filter.TenantID, Valid: true}
	}
	from, before := pgtype.Timestamptz{}, pgtype.Timestamptz{}
	if !filter.From.IsZero() {
		from = pgtype.Timestamptz{Time: filter.From.UTC(), Valid: true}
	}
	if !filter.Before.IsZero() {
		before = pgtype.Timestamptz{Time: filter.Before.UTC(), Valid: true}
	}
	rows, e := t.commercial.ListFilteredPlatformAIUsage(c, q.ListFilteredPlatformAIUsageParams{TenantFilter: tenant, KindFilter: queueFilter(filter.Kind), FromAt: from, BeforeAt: before, CursorAt: at, CursorID: id, PageLimit: limit})
	if e != nil {
		return nil, e
	}
	out := make([]app.PlatformAIUsageItem, 0, len(rows))
	for _, r := range rows {
		if !r.ID.Valid || !r.TenantID.Valid || r.TenantName == "" || r.ActualCredits == nil || *r.ActualCredits < 1 || !r.SettledAt.Valid || !domain.ValidAIUsageKind(domain.AIUsageKind(r.Kind)) {
			return nil, errors.New("ai_usage_corrupt")
		}
		out = append(out, app.PlatformAIUsageItem{ID: domain.ID(r.ID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), TenantName: r.TenantName, Kind: domain.AIUsageKind(r.Kind), ActualCredits: *r.ActualCredits, SettledAt: r.SettledAt.Time.UTC()})
	}
	return out, nil
}
