package integration

import (
	app "bidora/backend/internal/controlplane/application"
	q "bidora/backend/internal/controlplane/persistence/sqlc"
	"context"
	"time"
)

func (t *overviewTx) LookupReferences(c context.Context, kind, search string) ([]app.PlatformReference, error) {
	out := []app.PlatformReference{}
	if kind == "organizations" {
		rows, e := t.q.LookupPlatformOrganizations(c, search)
		if e != nil {
			return nil, e
		}
		for _, r := range rows {
			out = append(out, app.PlatformReference{ID: app.ID(r.ID.Bytes), Label: r.Label})
		}
	} else {
		rows, e := t.q.LookupPlatformSupportAssignees(c, search)
		if e != nil {
			return nil, e
		}
		for _, r := range rows {
			out = append(out, app.PlatformReference{ID: app.ID(r.ID.Bytes), Label: r.Label})
		}
	}
	return out, nil
}
func (t *overviewTx) AppendReferenceRead(c context.Context, id app.ID, s app.PlatformAuditSubject, at time.Time) error {
	return t.q.InsertPlatformReferenceReadAudit(c, q.InsertPlatformReferenceReadAuditParams{ID: supportUUID(id), ActorID: supportUUID(s.UserID), CorrelationID: supportUUID(s.CorrelationID), At: supportTime(at)})
}
