package platformaudit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	control "bidora/backend/internal/controlplane/application"
	platform "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
)

type ids struct{}

func (ids) NewID() (control.ID, error) { return control.ID{9}, nil }

type clock struct{ now time.Time }

func (c clock) Now() time.Time { return c.now }

type uow struct {
	tx        *tx
	committed bool
}

func (u *uow) WithinPlatformAudit(ctx context.Context, fn func(context.Context, control.PlatformAuditTx) error) error {
	if err := fn(ctx, u.tx); err != nil {
		return err
	}
	u.committed = true
	return nil
}

type tx struct {
	metadataRead           bool
	filter                 control.PlatformAuditFilter
	facts                  platform.AuthorityFacts
	rows                   []control.PlatformAuditAction
	listed, exported, read int
}

func (t *tx) ResolvePlatformAuthority(_ context.Context, _ platform.AuthorityQuery) (platform.AuthorityFacts, error) {
	return t.facts, nil
}
func (t *tx) ListPlatformAuditActions(_ context.Context, limit int32, filter control.PlatformAuditFilter, _ *control.PlatformAuditCursor) ([]control.PlatformAuditAction, error) {
	t.listed++
	t.filter = filter
	if limit != 501 && (limit < 2 || limit > 101) {
		return nil, errors.New("unbounded query")
	}
	return t.rows, nil
}
func (t *tx) GetPlatformAuditAction(context.Context, control.ID, bool) (control.PlatformAuditAction, bool, error) {
	return control.PlatformAuditAction{}, false, nil
}
func (t *tx) AppendPlatformAuditRead(_ context.Context, _ control.ID, subject control.PlatformAuditSubject, _ time.Time) error {
	t.metadataRead = subject.MetadataAccess
	t.read++
	return nil
}
func (t *tx) AppendPlatformAuditExport(context.Context, control.ID, control.PlatformAuditSubject, time.Time) error {
	t.exported++
	return nil
}

func TestPlatformAuditExportRequiresGrantBoundedWindowAndAtomicEvidence(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	subject := control.PlatformAuditSubject{UserID: control.ID{1}, SessionID: control.ID{2}, CorrelationID: control.ID{3}, ContextGeneration: 4}
	filter := control.PlatformAuditFilter{Sort: "DESC", From: now.Add(-24 * time.Hour), Before: now}
	facts := platform.AuthorityFacts{UserID: platform.ID(subject.UserID), ContextGeneration: 4, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: now.Add(-time.Minute), Roles: []domain.Role{domain.BillingAdministrator}}
	transaction := &tx{facts: facts}
	unit := &uow{tx: transaction}
	service, err := control.NewPlatformAuditService(unit, ids{}, clock{now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Export(context.Background(), subject, filter); !errors.Is(err, control.ErrPlatformAuditForbidden) || transaction.listed != 0 || transaction.exported != 0 {
		t.Fatalf("export permission bypass: %v", err)
	}
	transaction.facts.Roles = []domain.Role{domain.SecurityAuditAdministrator}
	if _, err = service.Export(context.Background(), subject, control.PlatformAuditFilter{Sort: "DESC"}); !errors.Is(err, control.ErrPlatformAuditInvalid) {
		t.Fatalf("unbounded export: %v", err)
	}
	transaction.rows = make([]control.PlatformAuditAction, 501)
	if _, err = service.Export(context.Background(), subject, filter); !errors.Is(err, control.ErrPlatformAuditInvalid) || unit.committed || transaction.exported != 0 {
		t.Fatalf("partial export committed: %v", err)
	}
	transaction.rows = transaction.rows[:1]
	page, err := service.Export(context.Background(), subject, filter)
	if err != nil || len(page.Actions) != 1 || !unit.committed || transaction.exported != 1 || transaction.read != 0 {
		t.Fatalf("export not atomically evidenced: %v", err)
	}
}
