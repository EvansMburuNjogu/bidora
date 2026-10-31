package opportunity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/transport"
)

func appID(n byte) application.ID { var id application.ID; id[15] = n; return id }

type authFake struct {
	authority application.CaptureAuthority
	err       error
	calls     int
}

func (a *authFake) AuthorizeCapture(context.Context, application.CaptureSubject) (application.CaptureAuthority, error) {
	a.calls++
	return a.authority, a.err
}

type documentFake struct {
	err  error
	refs []string
}

func (d *documentFake) CheckCaptureSource(_ context.Context, _ application.CaptureAuthority, ref string) error {
	d.refs = append(d.refs, ref)
	return d.err
}

type idsFake struct {
	next  byte
	errAt int
	calls int
}

func (g *idsFake) NewID() (application.ID, error) {
	g.calls++
	if g.calls == g.errAt {
		return application.ID{}, errors.New("secret-canary id diagnostic")
	}
	g.next++
	return appID(g.next), nil
}

type clockFake struct{ at time.Time }

func (c clockFake) Now() time.Time { return c.at }

type uowFake struct {
	tx        application.CaptureTx
	err       error
	calls     int
	authority application.CaptureAuthority
}

func (u *uowFake) WithinCapture(ctx context.Context, a application.CaptureAuthority, fn func(application.CaptureTx) error) error {
	u.calls++
	u.authority = a
	if u.err != nil {
		return u.err
	}
	return fn(u.tx)
}

type txFake struct {
	claim   application.CaptureClaim
	fail    string
	calls   []string
	command application.CaptureCommand
	keyHash [32]byte
}

func (t *txFake) step(name string) error {
	t.calls = append(t.calls, name)
	if t.fail == name {
		return errors.New("secret-canary persistence diagnostic")
	}
	return nil
}
func (t *txFake) Claim(_ context.Context, _ application.ID, _ application.ID, keyHash [32]byte, _ [32]byte, _ time.Time) (application.CaptureClaim, error) {
	t.keyHash = keyHash
	if err := t.step("claim"); err != nil {
		return application.CaptureClaim{}, err
	}
	return t.claim, nil
}
func (t *txFake) InsertOpportunity(_ context.Context, _, _, _ application.ID, c application.CaptureCommand, _ time.Time) error {
	t.command = c
	return t.step("insert")
}
func (t *txFake) AppendAudit(context.Context, application.ID, application.ID, application.ID, application.ID, time.Time) error {
	return t.step("audit")
}
func (t *txFake) AppendOutbox(context.Context, application.ID, application.ID, application.ID, application.ID, time.Time) error {
	return t.step("outbox")
}
func (t *txFake) Complete(context.Context, application.ID, [32]byte, application.ID, time.Time) error {
	return t.step("complete")
}

func captureCommand(t *testing.T, document bool) application.CaptureCommand {
	t.Helper()
	body := `{"title":"Synthetic","buyer_name":null}`
	if document {
		body = `{"title":"Synthetic","buyer_name":null,"source_document_version_id":"019c0000-0000-7000-8000-000000000001"}`
	}
	command, err := transport.DecodeCaptureJSON([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return command
}
func executor(t *testing.T, tx *txFake) (application.CaptureExecutor, *authFake, *documentFake, *idsFake, *uowFake) {
	t.Helper()
	auth := &authFake{authority: application.CaptureAuthority{ActorID: appID(20), TenantID: appID(21), AssignmentGeneration: 3}}
	docs := &documentFake{}
	ids := &idsFake{}
	uow := &uowFake{tx: tx}
	return application.CaptureExecutor{Authorizer: auth, Documents: docs, UOW: uow, IDs: ids, Clock: clockFake{time.Date(2026, 10, 18, 12, 0, 0, 0, time.FixedZone("source", 3600))}}, auth, docs, ids, uow
}
func executeInput(t *testing.T, document bool) application.ExecuteCaptureInput {
	return application.ExecuteCaptureInput{Subject: application.CaptureSubject{IdentityID: appID(10), SelectedTenantID: appID(11), ContextGeneration: 2}, IdempotencyKey: "019c0000-0000-7000-8000-000000000099", CorrelationID: appID(12), Command: captureCommand(t, document)}
}

func TestExecuteCaptureNewTransaction(t *testing.T) {
	command := captureCommand(t, false)
	hash, _ := command.Fingerprint()
	tx := &txFake{claim: application.CaptureClaim{State: application.IdempotencyNew, RequestHash: hash}}
	e, auth, docs, ids, uow := executor(t, tx)
	input := executeInput(t, false)
	input.Command = command
	result, err := e.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.OpportunityID != appID(2) || result.SourceVersionID != appID(3) || result.Version != 1 || result.Replayed || !result.CreatedAt.Equal(time.Date(2026, 10, 18, 11, 0, 0, 0, time.UTC)) {
		t.Fatal("incorrect capture result", result)
	}
	if strings.Join(tx.calls, ",") != "claim,insert,audit,outbox,complete" || auth.calls != 1 || len(docs.refs) != 0 || ids.calls != 5 || uow.calls != 1 || uow.authority.AssignmentGeneration != 3 {
		t.Fatal("incorrect orchestration", tx.calls)
	}
	facts, ok := tx.command.Facts()
	if !ok || facts.Title() != "Synthetic" || facts.Verified() {
		t.Fatal("transaction received altered command")
	}
}

func TestExecuteCaptureReplayReauthorizesAndRechecksDocument(t *testing.T) {
	command := captureCommand(t, true)
	hash, _ := command.Fingerprint()
	created := time.Date(2026, 10, 17, 9, 0, 0, 0, time.FixedZone("old", -7200))
	tx := &txFake{claim: application.CaptureClaim{State: application.IdempotencyCompleted, RequestHash: hash, OpportunityID: appID(40), SourceVersionID: appID(41), CreatedAt: created}}
	e, auth, docs, _, _ := executor(t, tx)
	input := executeInput(t, true)
	input.Command = command
	result, err := e.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.OpportunityID != appID(40) || !result.CreatedAt.Equal(created.UTC()) || strings.Join(tx.calls, ",") != "claim" || auth.calls != 1 || len(docs.refs) != 1 {
		t.Fatal("unsafe replay", result, tx.calls, docs.refs)
	}
}

func TestExecuteCaptureIdempotencyUUIDCaseIsCanonical(t *testing.T) {
	command := captureCommand(t, false)
	hash, _ := command.Fingerprint()
	var keys [][32]byte
	for _, key := range []string{"019c0000-0000-7000-8000-0000000000af", "019C0000-0000-7000-8000-0000000000AF"} {
		tx := &txFake{claim: application.CaptureClaim{State: application.IdempotencyPending, RequestHash: hash}}
		e, _, _, _, _ := executor(t, tx)
		input := executeInput(t, false)
		input.Command = command
		input.IdempotencyKey = key
		_, err := e.Execute(context.Background(), input)
		if err == nil || err.Error() != "capture_operation_in_progress" {
			t.Fatal(err)
		}
		keys = append(keys, tx.keyHash)
	}
	if keys[0] != keys[1] {
		t.Fatal("same UUID spelling produced different idempotency scope")
	}
}

func TestExecuteCaptureConflictPendingAndInconsistent(t *testing.T) {
	command := captureCommand(t, false)
	hash, _ := command.Fingerprint()
	cases := []struct {
		name  string
		claim application.CaptureClaim
		code  string
	}{
		{"different body", application.CaptureClaim{State: application.IdempotencyCompleted, RequestHash: [32]byte{1}}, "capture_idempotency_conflict"},
		{"pending", application.CaptureClaim{State: application.IdempotencyPending, RequestHash: hash}, "capture_operation_in_progress"},
		{"completed missing result", application.CaptureClaim{State: application.IdempotencyCompleted, RequestHash: hash}, "capture_persistence_inconsistent"},
		{"unknown state", application.CaptureClaim{State: 99, RequestHash: hash}, "capture_persistence_inconsistent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx := &txFake{claim: tc.claim}
			e, _, _, _, _ := executor(t, tx)
			input := executeInput(t, false)
			input.Command = command
			result, err := e.Execute(context.Background(), input)
			if err == nil || err.Error() != tc.code || result != (application.CaptureResult{}) {
				t.Fatal("unsafe idempotency result", result, err)
			}
		})
	}
}

func TestExecuteCaptureStopsAtEveryBoundary(t *testing.T) {
	command := captureCommand(t, false)
	hash, _ := command.Fingerprint()
	cause := errors.New("secret-canary authorization diagnostic")
	t.Run("authorization", func(t *testing.T) {
		tx := &txFake{}
		e, a, _, _, u := executor(t, tx)
		a.err = cause
		_, err := e.Execute(context.Background(), executeInput(t, false))
		if err == nil || err.Error() != "capture_not_authorized" || !errors.Is(err, cause) || u.calls != 0 || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
			t.Fatal("authorization failure unsafe", err)
		}
	})
	t.Run("document", func(t *testing.T) {
		tx := &txFake{}
		e, _, d, _, u := executor(t, tx)
		d.err = cause
		_, err := e.Execute(context.Background(), executeInput(t, true))
		if err == nil || err.Error() != "capture_source_unavailable" || u.calls != 0 {
			t.Fatal("document failure crossed transaction", err)
		}
	})
	for _, step := range []string{"claim", "insert", "audit", "outbox", "complete"} {
		t.Run(step, func(t *testing.T) {
			tx := &txFake{claim: application.CaptureClaim{State: application.IdempotencyNew, RequestHash: hash}, fail: step}
			e, _, _, _, _ := executor(t, tx)
			input := executeInput(t, false)
			input.Command = command
			result, err := e.Execute(context.Background(), input)
			if err == nil || err.Error() != "capture_persistence_failed" || result != (application.CaptureResult{}) {
				t.Fatal("persistence failure reported success", result, err)
			}
			for _, call := range tx.calls {
				if call == "complete" && step != "complete" {
					t.Fatal("completed failed transaction")
				}
			}
		})
	}
}

func TestExecuteCaptureRejectsInvalidInfrastructureAndInput(t *testing.T) {
	validTx := &txFake{}
	e, _, _, ids, _ := executor(t, validTx)
	cases := []struct {
		name   string
		ctx    context.Context
		mutate func(*application.CaptureExecutor, *application.ExecuteCaptureInput)
		code   string
	}{
		{"nil context", nil, func(*application.CaptureExecutor, *application.ExecuteCaptureInput) {}, "capture_context_required"},
		{"missing dependency", context.Background(), func(e *application.CaptureExecutor, _ *application.ExecuteCaptureInput) { e.UOW = nil }, "capture_not_configured"},
		{"missing correlation", context.Background(), func(_ *application.CaptureExecutor, i *application.ExecuteCaptureInput) {
			i.CorrelationID = application.ID{}
		}, "capture_correlation_required"},
		{"bad key", context.Background(), func(_ *application.CaptureExecutor, i *application.ExecuteCaptureInput) {
			i.IdempotencyKey = "secret-canary"
		}, "capture_idempotency_key_invalid"},
		{"zero command", context.Background(), func(_ *application.CaptureExecutor, i *application.ExecuteCaptureInput) {
			i.Command = application.CaptureCommand{}
		}, "capture_command_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := e
			input := executeInput(t, false)
			tc.mutate(&copy, &input)
			_, err := copy.Execute(tc.ctx, input)
			if err == nil || err.Error() != tc.code || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("invalid input accepted/leaked", err)
			}
		})
	}
	ids.errAt = 1
	_, err := e.Execute(context.Background(), executeInput(t, false))
	if err == nil || err.Error() != "capture_id_generation_failed" || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("ID failure unsafe", err)
	}
}
