package notification_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
)

type templateClock struct{ at time.Time }

func (c templateClock) Now() time.Time { return c.at }

type templateIDs struct{ next byte }

func (g *templateIDs) NewID() (domain.ID, error) { g.next++; return domain.ID{g.next}, nil }

type templateFixture struct {
	value                                app.Template
	receipts                             map[string]app.TemplateReceipt
	owner, forbidden, inTx, failEvidence bool
	writes, evidence, tests              int
}

func (f *templateFixture) WithinTemplates(ctx context.Context, _ bool, fn func(context.Context, app.TemplateTx) error) error {
	f.inTx = true
	defer func() { f.inTx = false }()
	old := f.value
	w, e, tests := f.writes, f.evidence, f.tests
	receipts := map[string]app.TemplateReceipt{}
	for k, v := range f.receipts {
		receipts[k] = v
	}
	if err := fn(ctx, f); err != nil {
		f.value = old
		f.writes, f.evidence, f.tests = w, e, tests
		f.receipts = receipts
		return err
	}
	return nil
}
func (f *templateFixture) AuthorizeTemplates(_ context.Context, _ app.TemplateSubject, write bool, _ time.Time) error {
	if f.forbidden || write && !f.owner {
		return app.ErrTemplateForbidden
	}
	return nil
}
func (f *templateFixture) ListTemplates(context.Context) ([]app.Template, error) {
	return []app.Template{f.value}, nil
}
func (f *templateFixture) ReadTemplate(context.Context, string, bool) (app.Template, error) {
	return f.value, nil
}
func (f *templateFixture) WriteTemplate(_ context.Context, t app.Template, _ app.TemplateSubject, _ time.Time) error {
	f.value = t
	f.value.Version++
	f.writes++
	return nil
}
func (f *templateFixture) TemplateHistory(context.Context, string) ([]app.Template, error) {
	return []app.Template{f.value}, nil
}
func (f *templateFixture) FindTemplateCommand(_ context.Context, s app.TemplateSubject, op string, key [32]byte) (app.TemplateReceipt, bool, error) {
	v, ok := f.receipts[fmt.Sprintf("%s:%s:%x", s.ActorID, op, key)]
	return v, ok, nil
}
func (f *templateFixture) RecordTemplateCommand(_ context.Context, s app.TemplateSubject, op string, key [32]byte, r app.TemplateReceipt, _ time.Time) error {
	f.receipts[fmt.Sprintf("%s:%s:%x", s.ActorID, op, key)] = r
	if op == "TEST" {
		f.tests++
	}
	return nil
}
func (f *templateFixture) FinishTemplateTest(_ context.Context, s app.TemplateSubject, key [32]byte, result string) error {
	k := fmt.Sprintf("%s:TEST:%x", s.ActorID, key)
	r := f.receipts[k]
	r.State = "COMPLETED"
	r.Result = result
	f.receipts[k] = r
	return nil
}
func (f *templateFixture) CountTemplateTests(context.Context, app.TemplateSubject, time.Time) (int64, error) {
	return int64(f.tests), nil
}
func (f *templateFixture) AppendTemplateEvidence(_ context.Context, _, _ domain.ID, _ app.TemplateSubject, _ app.Template, _, _ string, _ time.Time) error {
	if f.failEvidence {
		return errors.New("synthetic evidence failure")
	}
	f.evidence++
	return nil
}

type templateSender struct {
	fixture *templateFixture
	calls   int
	err     error
	message app.RenderedTemplate
}

func (s *templateSender) SendTemplateTest(ctx context.Context, _ app.TemplateSubject, _ app.Template, m app.RenderedTemplate, _ string) (string, error) {
	if s.fixture.inTx {
		return "", errors.New("network inside transaction")
	}
	if _, ok := ctx.Deadline(); !ok {
		return "", errors.New("unbounded network")
	}
	s.calls++
	s.message = m
	if s.err != nil {
		return "", s.err
	}
	return "SMTP_ACCEPTED", nil
}
func templateUnit(t *testing.T) (*app.TemplateService, *templateFixture, *templateSender, app.TemplateSubject) {
	t.Helper()
	at := time.Now().UTC()
	f := &templateFixture{owner: true, value: app.Template{Key: "identity.password_recovery", Name: "Password recovery", Subject: "{{platform_name}} password recovery", Body: "Use {{token}} before {{expires_at}}.", Enabled: true, Version: 1, UpdatedAt: at}, receipts: map[string]app.TemplateReceipt{}}
	sender := &templateSender{fixture: f}
	service, err := app.NewTemplateService(f, &templateIDs{}, templateClock{at}, sender)
	if err != nil {
		t.Fatal(err)
	}
	return service, f, sender, app.TemplateSubject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, CorrelationID: domain.ID{3}, ContextGeneration: 1}
}
func TestNotificationMandatoryAndPlaceholderValidation(t *testing.T) {
	_, f, _, _ := templateUnit(t)
	if !app.ValidTemplate(f.value) {
		t.Fatal("valid template rejected")
	}
	for _, mutate := range []func(*app.Template){func(v *app.Template) { v.Enabled = false }, func(v *app.Template) { v.Body = "Use {{token}}." }, func(v *app.Template) { v.Body = "{{token}} {{expires_at}} {{unsupported}}" }, func(v *app.Template) { v.Subject = "Code: {{token}}" }, func(v *app.Template) { v.Subject = "Subject\r\nBcc: leak@example.test" }, func(v *app.Template) { v.Body = "{{token}} {{expires_at}} {{ platform_name }}" }, func(v *app.Template) { v.Key = "billing.fabricated" }} {
		bad := f.value
		mutate(&bad)
		if app.ValidTemplate(bad) {
			t.Fatal("unsafe template accepted")
		}
	}
	optional := app.Template{Key: "platform.invitation.reminder", Name: "Reminder", Enabled: false, Subject: "Invitation reminder", Body: "{{role}} invitation expires at {{expires_at}}", Version: 1}
	if !app.ValidTemplate(optional) {
		t.Fatal("optional preferences rejected")
	}
	if _, err := app.RenderTemplate(optional, app.TemplatePreviewValues()); !errors.Is(err, app.ErrTemplateSuppressed) {
		t.Fatal("disabled preference still delivered", err)
	}
}
func TestNotificationRenderingEscapesOnceAndRequiresRuntimeValues(t *testing.T) {
	_, f, _, _ := templateUnit(t)
	values := app.TemplatePreviewValues()
	values["platform_name"] = "<script>{{token}}</script>"
	rendered, err := app.RenderTemplate(f.value, values)
	if err != nil || !strings.Contains(rendered.Subject, "{{token}}") || strings.Contains(rendered.HTML, "<script>") || !strings.Contains(rendered.Text, "EXAMPLE-TOKEN-NOT-VALID") {
		t.Fatal("rendering recursive or unsafe", err)
	}
	f.value.Body = "{{platform_name}}: {{token}} until {{expires_at}}"
	rendered, err = app.RenderTemplate(f.value, values)
	if err != nil || !strings.Contains(rendered.HTML, "&lt;script&gt;") {
		t.Fatal("HTML output not escaped", err)
	}
	delete(values, "token")
	if _, err = app.RenderTemplate(f.value, values); !errors.Is(err, app.ErrTemplateInvalid) {
		t.Fatal("missing runtime token accepted", err)
	}
	if strings.Contains(fmt.Sprintf("%#v", rendered), "EXAMPLE-TOKEN") {
		t.Fatal("rendered message formatting exposed content")
	}
}
func TestManagedNotificationFingerprintUsesStableSemanticContent(t *testing.T) {
	_, f, _, _ := templateUnit(t)
	a, err := app.RenderTemplate(f.value, app.TemplatePreviewValues())
	if err != nil {
		t.Fatal(err)
	}
	b, err := app.RenderTemplate(f.value, app.TemplatePreviewValues())
	if err != nil || !bytes.Equal(a.ContentForFingerprint(), b.ContentForFingerprint()) {
		t.Fatal("identical templates produced unstable idempotency content", err)
	}
	b.Text += " Changed"
	if bytes.Equal(a.ContentForFingerprint(), b.ContentForFingerprint()) {
		t.Fatal("changed rendered content reused fingerprint input")
	}
}

func TestNotificationSaveReplayConflictOwnerAndEvidence(t *testing.T) {
	s, f, _, subject := templateUnit(t)
	in := app.TemplateSave{Subject: subject, Template: f.value, IdempotencyKey: "save-1"}
	in.Template.Name = "Updated recovery"
	saved, err := s.Save(context.Background(), in)
	if err != nil || saved.Version != 2 || f.writes != 1 || f.evidence != 1 {
		t.Fatal("template not saved atomically", err)
	}
	if _, err = s.Save(context.Background(), in); err != nil || f.writes != 1 {
		t.Fatal("save replay repeated effects", err)
	}
	in.Template.Name = "Changed replay"
	if _, err = s.Save(context.Background(), in); !errors.Is(err, app.ErrTemplateConflict) {
		t.Fatal("changed replay accepted", err)
	}
	in.Template = f.value
	in.IdempotencyKey = "save-2"
	f.failEvidence = true
	if _, err = s.Save(context.Background(), in); !errors.Is(err, app.ErrTemplateUnavailable) || f.value.Version != 2 || f.writes != 1 {
		t.Fatal("unit rollback left template effects", err)
	}
	f.failEvidence = false
	f.owner = false
	if _, err = s.Save(context.Background(), in); !errors.Is(err, app.ErrTemplateForbidden) {
		t.Fatal("non-owner changed template", err)
	}
	f.forbidden = true
	if _, err = s.List(context.Background(), subject); !errors.Is(err, app.ErrTemplateForbidden) {
		t.Fatal("unauthorised template read", err)
	}
}
func TestNotificationSelfTestTruthfulOutcomeReplayAndRateBound(t *testing.T) {
	s, f, sender, subject := templateUnit(t)
	result, err := s.Test(context.Background(), subject, f.value.Key, 1, "test-1")
	if err != nil || result != "SMTP_ACCEPTED" || sender.calls != 1 || !strings.HasPrefix(sender.message.Subject, "Test notification: ") || !strings.Contains(sender.message.Text, "EXAMPLE-TOKEN-NOT-VALID") {
		t.Fatal("self-test did not use bounded synthetic delivery outside transaction", result, err)
	}
	if _, err = s.Test(context.Background(), subject, f.value.Key, 1, "test-1"); err != nil || sender.calls != 1 {
		t.Fatal("test replay resent notification", err)
	}
	sender.err = errors.New("synthetic SMTP uncertainty")
	result, err = s.Test(context.Background(), subject, f.value.Key, 1, "test-2")
	if err != nil || result != "UNAVAILABLE" {
		t.Fatal("uncertain send claimed accepted", result, err)
	}
	f.tests = 3
	if _, err = s.Test(context.Background(), subject, f.value.Key, 1, "test-4"); !errors.Is(err, app.ErrTemplateConflict) || sender.calls != 2 {
		t.Fatal("self-test rate bound bypassed", err)
	}
}
