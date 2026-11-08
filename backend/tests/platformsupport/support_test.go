package platformsupport_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/controlplane/application"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
)

type ids struct{ next byte }

func (i *ids) NewID() (app.ID, error) { i.next++; return app.ID{i.next}, nil }

type clock struct{ at time.Time }

func (c clock) Now() time.Time { return c.at }

type tx struct {
	app.SupportTx
	facts                             platformapp.AuthorityFacts
	caseRow                           app.SupportCase
	command                           app.SupportCommandRecord
	hasCommand                        bool
	events, audits, messages, updates int
}

func (t *tx) ResolvePlatformAuthority(context.Context, platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error) {
	return t.facts, nil
}
func (t *tx) TenantExists(context.Context, app.ID) (bool, error) { return true, nil }
func (t *tx) FindCommand(context.Context, app.ID, string, [32]byte) (app.SupportCommandRecord, bool, error) {
	return t.command, t.hasCommand, nil
}
func (t *tx) InsertCase(_ context.Context, v app.SupportCase) error { t.caseRow = v; return nil }
func (t *tx) ReadCase(_ context.Context, id app.ID, _ bool) (app.SupportCase, bool, error) {
	return t.caseRow, t.caseRow.ID == id, nil
}
func (t *tx) UpdateCase(_ context.Context, v app.SupportCase, _ int64) (bool, error) {
	t.caseRow = v
	t.updates++
	return true, nil
}
func (t *tx) AppendEvent(context.Context, app.ID, app.ID, app.ID, string, int64, time.Time) error {
	t.events++
	return nil
}
func (t *tx) AppendCommand(_ context.Context, _ app.ID, _ string, _ [32]byte, hash [32]byte, id app.ID, version int64, _ time.Time) error {
	t.hasCommand = true
	t.command = app.SupportCommandRecord{RequestHash: hash, CaseID: id, Version: version}
	return nil
}
func (t *tx) AppendAudit(context.Context, app.ID, app.ID, app.ID, app.ID, string, time.Time) error {
	t.audits++
	return nil
}
func (t *tx) AssigneeEligible(context.Context, app.ID) (bool, error) { return false, nil }
func (t *tx) MessageCount(context.Context, app.ID) (int64, error)    { return 0, nil }
func (t *tx) InsertMessage(context.Context, app.ID, app.SupportMessage) error {
	t.messages++
	return nil
}

type uow struct {
	tx      *tx
	commits int
}

func (u *uow) WithinSupport(ctx context.Context, fn func(context.Context, app.SupportTx) error) error {
	if err := fn(ctx, u.tx); err != nil {
		return err
	}
	u.commits++
	return nil
}
func TestSupportProjectionKeepsControlBoundary(t *testing.T) {
	body, err := os.ReadFile("../../migrations/control/00045_platform_support_cases.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, clause := range []string{"e.tenant_id IS NULL", "e.target_type='platform_support_page' AND e.action='PlatformSupportCasesRead'", "e.target_type='platform_support_case' AND e.action IN", "CASE WHEN e.target_type IN", "support_messages_immutable", "support_events_immutable", "support_commands_immutable"} {
		if !strings.Contains(string(body), clause) {
			t.Fatalf("support isolation or evidence clause missing: %s", clause)
		}
	}
}
func TestSupportLifecycleAuthorityVersionReplayAndEvidence(t *testing.T) {
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	subject := app.PlatformAuditSubject{UserID: app.ID{1}, SessionID: app.ID{2}, CorrelationID: app.ID{3}, ContextGeneration: 2}
	transaction := &tx{facts: platformapp.AuthorityFacts{UserID: platformapp.ID(subject.UserID), ContextGeneration: 2, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: now.Add(-time.Minute), Roles: []domain.Role{domain.BillingAdministrator}}}
	unit := &uow{tx: transaction}
	service, err := app.NewSupportService(unit, &ids{}, clock{now})
	if err != nil {
		t.Fatal(err)
	}
	create := app.SupportCommand{Subject: subject, Operation: "CREATE", TenantID: app.ID{4}, Title: "Case title", Description: "Control-only support request", IdempotencyKey: "synthetic-key"}
	if _, err = service.Command(context.Background(), create); !errors.Is(err, app.ErrSupportForbidden) || unit.commits != 0 {
		t.Fatalf("billing-only authorization bypass: %v", err)
	}
	// Fixed Platform Owner explicitly includes Support capabilities; no tenant membership is conferred.
	transaction.facts.Roles = []domain.Role{domain.SupportAdministrator}
	receipt, err := service.Command(context.Background(), create)
	if err != nil || receipt.CaseID.IsZero() || receipt.Version != 1 || transaction.events != 1 || transaction.audits != 1 || unit.commits != 1 {
		t.Fatalf("create not atomically evidenced: %+v %v", receipt, err)
	}
	replay, err := service.Command(context.Background(), create)
	if err != nil || !replay.Replay || replay.Version != 1 || transaction.events != 1 {
		t.Fatalf("replay duplicated evidence: %+v %v", replay, err)
	}
	changed := create
	changed.Title = "Different title"
	if _, err = service.Command(context.Background(), changed); !errors.Is(err, app.ErrSupportConflict) {
		t.Fatalf("key reused for different request: %v", err)
	}
	transaction.hasCommand = false
	resolve := app.SupportCommand{Subject: subject, Operation: "RESOLVE", CaseID: receipt.CaseID, ExpectedVersion: 1, IdempotencyKey: "resolve-key"}
	next, err := service.Command(context.Background(), resolve)
	if err != nil || next.Version != 2 || transaction.caseRow.State != "RESOLVED" || transaction.updates != 1 || transaction.events != 2 {
		t.Fatalf("resolve not fenced or evidenced: %+v %v", next, err)
	}
	transaction.hasCommand = false
	if _, err = service.Command(context.Background(), resolve); !errors.Is(err, app.ErrSupportConflict) || transaction.events != 2 {
		t.Fatalf("stale version accepted: %v", err)
	}
}
