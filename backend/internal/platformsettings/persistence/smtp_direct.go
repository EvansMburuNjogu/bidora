package persistence

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	q "bidora/backend/internal/platformsettings/persistence/sqlc"
	"context"
	"time"
)

func (t *tx) PublishSMTP(ctx context.Context, v domain.Version, head app.Head, actor, history domain.ID, at time.Time) error {
	if v.Kind != domain.SMTP || head.Kind != domain.SMTP || actor.IsZero() || history.IsZero() {
		return app.ErrInvalid
	}
	if err := t.q.RetireSMTPApprovalRequests(ctx, ts(at)); err != nil {
		return err
	}
	if err := t.q.SupersedePendingSMTPVersions(ctx, ts(at)); err != nil {
		return err
	}
	if !head.ActiveVersionID.IsZero() {
		n, e := t.q.SupersedeActiveIntegrationVersion(ctx, q.SupersedeActiveIntegrationVersionParams{ID: uuid(head.ActiveVersionID), At: ts(at)})
		if e != nil {
			return e
		}
		if n != 1 {
			return app.ErrConflict
		}
	}
	n, e := t.q.ActivateSMTPDirect(ctx, q.ActivateSMTPDirectParams{ID: uuid(v.ID), At: ts(at), ExpectedVersion: v.Version})
	if e != nil {
		return e
	}
	if n != 1 {
		return app.ErrConflict
	}
	n, e = t.q.AdvanceIntegrationHead(ctx, q.AdvanceIntegrationHeadParams{ActiveVersionID: uuid(v.ID), At: ts(at), SettingKind: string(domain.SMTP), ExpectedGeneration: head.Generation, ExpectedActiveVersionID: uuid(head.ActiveVersionID)})
	if e != nil {
		return e
	}
	if n != 1 {
		return app.ErrConflict
	}
	return t.q.InsertDirectSMTPHistory(ctx, q.InsertDirectSMTPHistoryParams{ID: uuid(history), VersionID: uuid(v.ID), PreviousVersionID: uuid(head.ActiveVersionID), Generation: head.Generation + 1, ActorID: uuid(actor), At: ts(at)})
}
