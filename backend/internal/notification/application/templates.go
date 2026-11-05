package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"bidora/backend/internal/emailhtml"
	"bidora/backend/internal/notification/domain"
)

var ErrTemplateInvalid = errors.New("notification_template_invalid")
var ErrTemplateForbidden = errors.New("notification_template_forbidden")
var ErrTemplateConflict = errors.New("notification_template_conflict")
var ErrTemplateUnavailable = errors.New("notification_template_unavailable")
var ErrTemplateSuppressed = errors.New("notification_template_suppressed")

type TemplateSubject struct {
	ActorID, SessionID, CorrelationID domain.ID
	ContextGeneration                 int64
}
type Template struct {
	Key           string    `json:"template_key"`
	Name          string    `json:"name"`
	Category      string    `json:"category"`
	Channel       string    `json:"channel"`
	Mandatory     bool      `json:"mandatory"`
	Enabled       bool      `json:"enabled"`
	Subject       string    `json:"subject"`
	Body          string    `json:"body"`
	Version       int64     `json:"version"`
	UpdatedAt     time.Time `json:"updated_at"`
	Placeholders  []string  `json:"placeholders"`
	TestAvailable bool      `json:"test_available"`
}
type TemplateSave struct {
	Subject        TemplateSubject
	Template       Template
	IdempotencyKey string
}
type TemplateReceipt struct {
	Hash          [32]byte
	Key           string
	Version       int64
	State, Result string
}
type TemplateTx interface {
	AuthorizeTemplates(context.Context, TemplateSubject, bool, time.Time) error
	ListTemplates(context.Context) ([]Template, error)
	ReadTemplate(context.Context, string, bool) (Template, error)
	WriteTemplate(context.Context, Template, TemplateSubject, time.Time) error
	TemplateHistory(context.Context, string) ([]Template, error)
	FindTemplateCommand(context.Context, TemplateSubject, string, [32]byte) (TemplateReceipt, bool, error)
	RecordTemplateCommand(context.Context, TemplateSubject, string, [32]byte, TemplateReceipt, time.Time) error
	FinishTemplateTest(context.Context, TemplateSubject, [32]byte, string) error
	CountTemplateTests(context.Context, TemplateSubject, time.Time) (int64, error)
	AppendTemplateEvidence(context.Context, domain.ID, domain.ID, TemplateSubject, Template, string, string, time.Time) error
}
type TemplateUnitOfWork interface {
	WithinTemplates(context.Context, bool, func(context.Context, TemplateTx) error) error
}
type TemplateTestSender interface {
	SendTemplateTest(context.Context, TemplateSubject, Template, RenderedTemplate, string) (string, error)
}
type TemplateService struct {
	uow    TemplateUnitOfWork
	ids    IDGenerator
	clock  Clock
	tester TemplateTestSender
}

func NewTemplateService(uow TemplateUnitOfWork, ids IDGenerator, clock Clock, tester TemplateTestSender) (*TemplateService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrTemplateInvalid
	}
	return &TemplateService{uow, ids, clock, tester}, nil
}

type templateDefinition struct {
	category               string
	mandatory              bool
	placeholders, required []string
}

var templateDefinitions = map[string]templateDefinition{
	"identity.email_verification":  {"EMAIL_VERIFICATION", true, []string{"platform_name", "token", "expires_at"}, []string{"token", "expires_at"}},
	"identity.password_recovery":   {"PASSWORD_RECOVERY", true, []string{"platform_name", "token", "expires_at"}, []string{"token", "expires_at"}},
	"identity.login_code":          {"EMAIL_LOGIN", true, []string{"platform_name", "code", "expires_at"}, []string{"code", "expires_at"}},
	"tenant.invitation":            {"TENANT_INVITATION", true, []string{"platform_name", "role", "token", "expires_at"}, []string{"token", "expires_at", "role"}},
	"platform.invitation.issued":   {"PLATFORM_INVITATION", true, []string{"platform_name", "role", "token", "expires_at"}, []string{"token", "expires_at", "role"}},
	"platform.invitation.reminder": {"PLATFORM_INVITATION_REMINDER", false, []string{"platform_name", "role", "expires_at"}, []string{"role", "expires_at"}},
	"platform.session_revocation":  {"PLATFORM_SESSION_REVOCATION", true, []string{"platform_name", "occurred_at"}, []string{"occurred_at"}},
	"platform.identity_state":      {"PLATFORM_IDENTITY_STATE_CHANGE", true, []string{"platform_name", "action", "occurred_at"}, []string{"action", "occurred_at"}},
	"platform.role_change":         {"PLATFORM_ROLE_CHANGE", true, []string{"platform_name", "role", "action", "occurred_at"}, []string{"role", "action", "occurred_at"}},
}

func SupportedTemplate(key string) bool { _, ok := templateDefinitions[key]; return ok }

var placeholderPattern = regexp.MustCompile(`\{\{([a-z_]+)\}\}`)

func ValidTemplate(t Template) bool {
	d, ok := templateDefinitions[t.Key]
	if !ok || !utf8.ValidString(t.Name+t.Subject+t.Body) || len(t.Name) < 1 || len(t.Name) > 80 || strings.TrimSpace(t.Name) != t.Name || len(t.Subject) < 1 || len(t.Subject) > 200 || strings.TrimSpace(t.Subject) != t.Subject || strings.IndexFunc(t.Subject, func(r rune) bool { return r < 32 || r == 127 }) >= 0 || len(t.Body) < 1 || len(t.Body) > 4000 || t.Version < 1 || !t.Enabled && d.mandatory {
		return false
	}
	allowed := map[string]bool{}
	for _, p := range d.placeholders {
		allowed[p] = true
	}
	for _, text := range []string{t.Subject, t.Body} {
		for _, match := range placeholderPattern.FindAllStringSubmatch(text, -1) {
			if !allowed[match[1]] {
				return false
			}
		}
		stripped := placeholderPattern.ReplaceAllString(text, "")
		if strings.Contains(stripped, "{{") || strings.Contains(stripped, "}}") {
			return false
		}
	}
	if strings.Contains(t.Subject, "{{token}}") || strings.Contains(t.Subject, "{{code}}") {
		return false
	}
	for _, p := range d.required {
		if !strings.Contains(t.Body, "{{"+p+"}}") {
			return false
		}
	}
	return true
}
func (s *TemplateService) decorate(t Template) Template {
	if d, ok := templateDefinitions[t.Key]; ok {
		t.Category = d.category
		t.Mandatory = d.mandatory
		t.Channel = "EMAIL"
		t.Placeholders = append([]string(nil), d.placeholders...)
	}
	t.TestAvailable = s.tester != nil
	return t
}
func validTemplateSubject(s TemplateSubject) bool {
	return !s.ActorID.IsZero() && !s.SessionID.IsZero() && s.ContextGeneration > 0
}
func templateError(err error) error {
	if err == nil || errors.Is(err, ErrTemplateInvalid) || errors.Is(err, ErrTemplateForbidden) || errors.Is(err, ErrTemplateConflict) || errors.Is(err, ErrTemplateSuppressed) {
		return err
	}
	return ErrTemplateUnavailable
}
func (s *TemplateService) List(ctx context.Context, subject TemplateSubject) (out []Template, err error) {
	if s == nil || ctx == nil || !validTemplateSubject(subject) {
		return nil, ErrTemplateInvalid
	}
	err = s.uow.WithinTemplates(ctx, false, func(c context.Context, tx TemplateTx) error {
		if e := tx.AuthorizeTemplates(c, subject, false, s.clock.Now()); e != nil {
			return e
		}
		rows, e := tx.ListTemplates(c)
		for _, t := range rows {
			if !SupportedTemplate(t.Key) || !ValidTemplate(t) {
				return ErrTemplateUnavailable
			}
			out = append(out, s.decorate(t))
		}
		return e
	})
	if out == nil {
		out = []Template{}
	}
	return out, templateError(err)
}
func (s *TemplateService) Read(ctx context.Context, subject TemplateSubject, key string) (out Template, err error) {
	if s == nil || ctx == nil || !validTemplateSubject(subject) || !SupportedTemplate(key) {
		return out, ErrTemplateInvalid
	}
	err = s.uow.WithinTemplates(ctx, false, func(c context.Context, tx TemplateTx) error {
		if e := tx.AuthorizeTemplates(c, subject, false, s.clock.Now()); e != nil {
			return e
		}
		var e error
		out, e = tx.ReadTemplate(c, key, false)
		return e
	})
	return s.decorate(out), templateError(err)
}
func (s *TemplateService) History(ctx context.Context, subject TemplateSubject, key string) (out []Template, err error) {
	if s == nil || ctx == nil || !validTemplateSubject(subject) || !SupportedTemplate(key) {
		return nil, ErrTemplateInvalid
	}
	err = s.uow.WithinTemplates(ctx, false, func(c context.Context, tx TemplateTx) error {
		if e := tx.AuthorizeTemplates(c, subject, false, s.clock.Now()); e != nil {
			return e
		}
		rows, e := tx.TemplateHistory(c, key)
		for _, t := range rows {
			out = append(out, s.decorate(t))
		}
		return e
	})
	if out == nil {
		out = []Template{}
	}
	return out, templateError(err)
}
func (s *TemplateService) Save(ctx context.Context, in TemplateSave) (out Template, err error) {
	if s == nil || ctx == nil || !validTemplateSubject(in.Subject) || in.Subject.CorrelationID.IsZero() || !ValidTemplate(in.Template) || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 128 {
		return out, ErrTemplateInvalid
	}
	at := s.clock.Now().UTC()
	t := s.decorate(in.Template)
	encoded, _ := json.Marshal(struct {
		Key, Name, Subject, Body string
		Enabled                  bool
		Version                  int64
	}{t.Key, t.Name, t.Subject, t.Body, t.Enabled, t.Version})
	hash := sha256.Sum256(encoded)
	key := sha256.Sum256([]byte("template-save-v1\x00" + in.IdempotencyKey))
	err = s.uow.WithinTemplates(ctx, true, func(c context.Context, tx TemplateTx) error {
		if e := tx.AuthorizeTemplates(c, in.Subject, true, at); e != nil {
			return e
		}
		current, e := tx.ReadTemplate(c, t.Key, true)
		if e != nil {
			return e
		}
		old, found, e := tx.FindTemplateCommand(c, in.Subject, "SAVE", key)
		if e != nil {
			return e
		}
		if found {
			if old.Hash != hash || old.Key != t.Key || old.Version != current.Version {
				return ErrTemplateConflict
			}
			out = s.decorate(current)
			return nil
		}
		if current.Version != t.Version {
			return ErrTemplateConflict
		}
		if e = tx.WriteTemplate(c, t, in.Subject, at); e != nil {
			return e
		}
		t.Version++
		t.UpdatedAt = at
		if e = tx.RecordTemplateCommand(c, in.Subject, "SAVE", key, TemplateReceipt{Hash: hash, Key: t.Key, Version: t.Version, State: "COMPLETED", Result: "SAVED"}, at); e != nil {
			return e
		}
		if e = s.evidence(c, tx, in.Subject, t, "NotificationTemplateUpdated", "SAVED", at); e != nil {
			return e
		}
		out = t
		return nil
	})
	return out, templateError(err)
}

type TemplateRenderer interface {
	Render(context.Context, string, map[string]string) (RenderedTemplate, error)
}

type RenderedTemplate struct {
	Subject, Text, HTML string
	Version             int64
	Category, Key       string
}

func (RenderedTemplate) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED rendered notification]")
}

// ContentForFingerprint excludes transport-generated MIME boundaries and dates.
// The recorder separately binds recipient, template version, source and SMTP version.
func (r RenderedTemplate) ContentForFingerprint() []byte {
	encoded, _ := json.Marshal([3]string{r.Subject, r.Text, r.HTML})
	return encoded
}

func RenderTemplate(t Template, values map[string]string) (RenderedTemplate, error) {
	if !ValidTemplate(t) {
		return RenderedTemplate{}, ErrTemplateInvalid
	}
	if !t.Enabled {
		return RenderedTemplate{}, ErrTemplateSuppressed
	}
	render := func(text string) (string, error) {
		missing := false
		out := placeholderPattern.ReplaceAllStringFunc(text, func(m string) string {
			k := placeholderPattern.FindStringSubmatch(m)[1]
			v, ok := values[k]
			if !ok || len(v) > 2048 {
				missing = true
			}
			return v
		})
		if missing {
			return "", ErrTemplateInvalid
		}
		return out, nil
	}
	subject, e := render(t.Subject)
	if e != nil || len(subject) > 512 || strings.ContainsAny(subject, "\r\n\x00") {
		return RenderedTemplate{}, ErrTemplateInvalid
	}
	body, e := render(t.Body)
	if e != nil {
		return RenderedTemplate{}, e
	}
	if t.Key == "identity.password_recovery" {
		body += "\n\nOpen the Reset password link, then enter the recovery code above, your new password, and confirm your new password. This code can only be used once. If you did not request a password reset, ignore this email."
	}
	body += "\n\nIf you did not expect this notification, contact your authorised support channel. Never share passwords or sign-in codes."
	displayBody := strings.TrimSuffix(body, "\n\nIf you did not expect this notification, contact your authorised support channel. Never share passwords or sign-in codes.")
	if action := emailhtml.QuickAction(values["web_origin"], t.Key); action.URL != "" {
		body += "\n\n" + action.Label + ": " + action.URL
	}
	code := values["code"]
	if code == "" {
		code = values["token"]
	}
	if code != "" {
		displayBody = strings.ReplaceAll(displayBody, code, "(shown below)")
	}
	return RenderedTemplate{Subject: subject, Text: body, HTML: emailhtml.Render(emailhtml.Content{Brand: values["platform_name"], Title: subject, Body: displayBody, Code: code, Expiry: values["expires_at"], Origin: values["web_origin"], Key: t.Key}), Version: t.Version, Category: templateDefinitions[t.Key].category, Key: t.Key}, nil
}
func (s *TemplateService) Render(ctx context.Context, key string, values map[string]string) (out RenderedTemplate, err error) {
	if s == nil || ctx == nil || !SupportedTemplate(key) {
		return out, ErrTemplateInvalid
	}
	var t Template
	err = s.uow.WithinTemplates(ctx, false, func(c context.Context, tx TemplateTx) error {
		var e error
		t, e = tx.ReadTemplate(c, key, false)
		return e
	})
	if err != nil {
		return out, templateError(err)
	}
	return RenderTemplate(s.decorate(t), values)
}
func TemplatePreviewValues() map[string]string {
	return map[string]string{"platform_name": "Bidora Preview", "token": "EXAMPLE-TOKEN-NOT-VALID", "code": "EXAMPLE-CODE", "expires_at": "2030-01-01T12:00:00Z", "occurred_at": "2030-01-01T11:00:00Z", "role": "Example role", "action": "changed"}
}
func (s *TemplateService) evidence(ctx context.Context, tx TemplateTx, subject TemplateSubject, t Template, action, result string, at time.Time) error {
	audit, e := s.ids.NewID()
	if e != nil {
		return e
	}
	outbox, e := s.ids.NewID()
	if e != nil {
		return e
	}
	return tx.AppendTemplateEvidence(ctx, audit, outbox, subject, t, action, result, at)
}
func (s *TemplateService) Test(ctx context.Context, subject TemplateSubject, key string, version int64, idempotency string) (result string, err error) {
	if s == nil || s.tester == nil || ctx == nil || !validTemplateSubject(subject) || subject.CorrelationID.IsZero() || !SupportedTemplate(key) || version < 1 || len(idempotency) < 1 || len(idempotency) > 128 {
		return "", ErrTemplateUnavailable
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", key, version)))
	receiptKey := sha256.Sum256([]byte("template-test-v1\x00" + idempotency))
	var t Template
	replay := false
	at := s.clock.Now().UTC()
	err = s.uow.WithinTemplates(ctx, true, func(c context.Context, tx TemplateTx) error {
		if e := tx.AuthorizeTemplates(c, subject, true, at); e != nil {
			return e
		}
		var e error
		t, e = tx.ReadTemplate(c, key, true)
		if e != nil {
			return e
		}
		if t.Version != version {
			return ErrTemplateConflict
		}
		old, found, e := tx.FindTemplateCommand(c, subject, "TEST", receiptKey)
		if e != nil {
			return e
		}
		if found {
			if old.Hash != hash {
				return ErrTemplateConflict
			}
			replay = true
			result = old.Result
			return nil
		}
		count, e := tx.CountTemplateTests(c, subject, at.Add(-10*time.Minute))
		if e != nil {
			return e
		}
		if count >= 3 {
			return ErrTemplateConflict
		}
		return tx.RecordTemplateCommand(c, subject, "TEST", receiptKey, TemplateReceipt{Hash: hash, Key: key, Version: version, State: "PENDING", Result: "PENDING"}, at)
	})
	if err != nil || replay {
		return result, templateError(err)
	}
	testTemplate := s.decorate(t)
	testTemplate.Enabled = true // explicit self-test, not an automatic delivery
	rendered, e := RenderTemplate(testTemplate, TemplatePreviewValues())
	if e != nil {
		return "", e
	}
	rendered.Subject = "Test notification: " + rendered.Subject
	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	result, e = s.tester.SendTemplateTest(sendCtx, subject, t, rendered, idempotency)
	cancel()
	if e != nil || result != "SMTP_ACCEPTED" {
		result = "UNAVAILABLE"
	}
	err = s.uow.WithinTemplates(ctx, true, func(c context.Context, tx TemplateTx) error {
		if e := tx.FinishTemplateTest(c, subject, receiptKey, result); e != nil {
			return e
		}
		return s.evidence(c, tx, subject, t, "NotificationTemplateTested", result, s.clock.Now())
	})
	return result, templateError(err)
}
