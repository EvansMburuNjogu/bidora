package httpapi

import (
	"context"
	"errors"

	settingsapp "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformIntegrationSettingsWriter interface {
	AuthorizeCreate(context.Context, settingsapp.Subject, domain.Kind) error
	Create(context.Context, settingsapp.CreateInput) (domain.Version, error)
	Test(context.Context, settingsapp.TestInput) (settingsapp.TestExecution, error)
	Propose(context.Context, settingsapp.ProposeInput) (domain.Change, error)
	Decide(context.Context, settingsapp.DecideInput) (domain.Change, error)
}

type PlatformSMTPSecretWriter interface {
	StoreSMTPSecrets(context.Context, domain.ID, string, string, string) (string, string, error)
}
type PlatformIntegrationSettingsWriteHandler struct {
	writer  PlatformIntegrationSettingsWriter
	reader  PlatformIntegrationSettingsReader
	clock   BrowserClock
	secrets PlatformSMTPSecretWriter
}

func NewPlatformIntegrationSettingsWriteHandler(writer PlatformIntegrationSettingsWriter, reader PlatformIntegrationSettingsReader, clock BrowserClock) (*PlatformIntegrationSettingsWriteHandler, error) {
	if writer == nil || reader == nil || clock == nil {
		return nil, errors.New("platform_integration_settings_write_handler_invalid")
	}
	return &PlatformIntegrationSettingsWriteHandler{writer: writer, reader: reader, clock: clock}, nil
}
func NewPlatformIntegrationSettingsWriteHandlerWithSecrets(writer PlatformIntegrationSettingsWriter, reader PlatformIntegrationSettingsReader, clock BrowserClock, secrets PlatformSMTPSecretWriter) (*PlatformIntegrationSettingsWriteHandler, error) {
	if secrets == nil {
		return nil, errors.New("platform_integration_settings_secret_writer_invalid")
	}
	h, err := NewPlatformIntegrationSettingsWriteHandler(writer, reader, clock)
	if err != nil {
		return nil, err
	}
	h.secrets = secrets
	return h, nil
}

// RegisterPlatformIntegrationSettingsWriteRoutes is deliberately separate from
// pilot composition. Callers must supply governed secret resolution and SMTP
// test composition before mounting it.
func RegisterPlatformIntegrationSettingsWriteRoutes(app *fiber.App, middleware *BrowserSessionMiddleware, handler *PlatformIntegrationSettingsWriteHandler) error {
	if app == nil || middleware == nil || handler == nil {
		return errors.New("platform_integration_settings_write_routes_invalid")
	}
	g := app.Group("/v1/platform", middleware.Platform)
	g.Get("/integration-settings/OPENAI/models", handler.models)
	g.Post("/integration-settings/:setting_kind/versions", handler.create)
	g.Post("/integration-settings/:setting_kind/versions/:version_id/test", handler.test)
	g.Post("/integration-settings/:setting_kind/versions/:version_id/activation-proposals", handler.propose)
	g.Post("/integration-setting-changes/:change_id/approve", handler.approve)
	g.Post("/integration-setting-changes/:change_id/reject", handler.reject)
	return nil
}

type createSMTPVersionRequest struct {
	RetainCredentials       bool    `json:"retain_credentials"`
	ExpectedActiveVersionID string  `json:"expected_active_version_id"`
	ExpectedActiveVersion   int64   `json:"expected_active_version"`
	SettingKind             string  `json:"setting_kind"`
	Host                    string  `json:"host"`
	Port                    int32   `json:"port"`
	Username                string  `json:"username"`
	Password                string  `json:"password"`
	TrustRootsPEM           string  `json:"trust_roots_pem"`
	SenderName              string  `json:"sender_name"`
	SenderAddress           string  `json:"sender_address"`
	ReplyToAddress          *string `json:"reply_to_address"`
	ConnectTimeoutSeconds   int32   `json:"connect_timeout_seconds"`
	CommandTimeoutSeconds   int32   `json:"command_timeout_seconds"`
	DeliveryTimeoutSeconds  int32   `json:"delivery_timeout_seconds"`
	MaxConcurrency          int32   `json:"max_concurrency"`
	Reason                  string  `json:"reason"`
}
type settingReasonRequest struct {
	ConfirmBillableTest bool   `json:"confirm_billable_test,omitempty"`
	Reason              string `json:"reason"`
}
type settingProposalRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type settingMutation struct {
	subject     settingsapp.Subject
	correlation domain.ID
	idempotency string
}

func (h *PlatformIntegrationSettingsWriteHandler) create(c fiber.Ctx) error {
	common, ok := h.mutation(c)
	if !ok {
		return nil
	}
	if c.Params("setting_kind") == string(domain.OpenAI) {
		return h.createOpenAI(c, common)
	}
	if c.Params("setting_kind") == string(domain.PesaPal) {
		return h.createPesaPal(c, common)
	}
	if c.Params("setting_kind") != string(domain.SMTP) {
		return fail(c, 404, "PLATFORM_SETTING_NOT_FOUND", "The setting was not found.", false)
	}
	var body createSMTPVersionRequest
	if err := bindWorkflowLimit(c, &body, 32*1024); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"setting_kind", "host", "port", "username", "password", "trust_roots_pem", "sender_name", "sender_address", "reply_to_address", "connect_timeout_seconds", "command_timeout_seconds", "delivery_timeout_seconds", "max_concurrency", "reason"}, "retain_credentials", "expected_active_version_id", "expected_active_version") || body.SettingKind != string(domain.SMTP) || h.secrets == nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if _, _, validationErr := domain.NewConfiguration(domain.SMTP, &domain.SMTPConfiguration{Host: body.Host, Port: body.Port, CredentialSecretRef: "pending", TrustRootsSecretRef: "pending", SenderName: body.SenderName, SenderAddress: body.SenderAddress, ReplyToAddress: body.ReplyToAddress, ConnectTimeoutSeconds: body.ConnectTimeoutSeconds, CommandTimeoutSeconds: body.CommandTimeoutSeconds, DeliveryTimeoutSeconds: body.DeliveryTimeoutSeconds, MaxConcurrency: body.MaxConcurrency}, nil); validationErr != nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if authorizeErr := h.writer.AuthorizeCreate(c.Context(), common.subject, domain.SMTP); authorizeErr != nil {
		return platformSettingFailure(c, authorizeErr)
	}
	var expectedID domain.ID
	if body.RetainCredentials {
		parsedID, idErr := uuid.Parse(body.ExpectedActiveVersionID)
		expectedID = domain.ID(parsedID)
		if idErr != nil || expectedID.IsZero() || body.ExpectedActiveVersion < 1 || body.Password != "" {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		reader, ok := h.secrets.(settingsapp.SMTPUsernameReader)
		if !ok {
			return platformSettingFailure(c, settingsapp.ErrUnavailable)
		}
		username, e := reader.SMTPUsername(c.Context(), expectedID)
		if e != nil {
			return platformSettingFailure(c, settingsapp.ErrUnavailable)
		}
		if username != body.Username {
			return fail(c, 400, "INVALID_REQUEST", "Enter a new password when changing the SMTP username.", false)
		}
	}
	responseUsername := body.Username
	var credentialRef, rootsRef string
	var secrets []settingsapp.SealedIntegrationSecret
	var secretErr error
	if body.RetainCredentials {
		credentialRef, rootsRef = "retained", "retained"
	} else if preparer, ok := h.secrets.(interface {
		PrepareSMTPSecrets(domain.ID, string, string, string, string) (string, string, []settingsapp.SealedIntegrationSecret, error)
	}); ok {
		credentialRef, rootsRef, secrets, secretErr = preparer.PrepareSMTPSecrets(common.subject.ActorID, common.idempotency, body.Username, body.Password, body.TrustRootsPEM)
	} else {
		credentialRef, rootsRef, secretErr = h.secrets.StoreSMTPSecrets(c.Context(), common.subject.ActorID, body.Username, body.Password, body.TrustRootsPEM)
	}
	body.Username = ""
	body.Password = ""
	body.TrustRootsPEM = ""
	if secretErr != nil {
		return fail(c, 503, "PLATFORM_SETTINGS_UNAVAILABLE", "Platform settings are temporarily unavailable.", true)
	}
	configuration, _, err := domain.NewConfiguration(domain.SMTP, &domain.SMTPConfiguration{Host: body.Host, Port: body.Port, CredentialSecretRef: credentialRef, TrustRootsSecretRef: rootsRef, SenderName: body.SenderName, SenderAddress: body.SenderAddress, ReplyToAddress: body.ReplyToAddress, ConnectTimeoutSeconds: body.ConnectTimeoutSeconds, CommandTimeoutSeconds: body.CommandTimeoutSeconds, DeliveryTimeoutSeconds: body.DeliveryTimeoutSeconds, MaxConcurrency: body.MaxConcurrency}, nil)
	if err != nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.writer.Create(c.Context(), settingsapp.CreateInput{ReuseSMTPSecrets: body.RetainCredentials, ExpectedSMTPVersionID: expectedID, ExpectedSMTPVersion: body.ExpectedActiveVersion, Subject: common.subject, Configuration: configuration, Secrets: secrets, Reason: body.Reason, IdempotencyKey: common.idempotency, CorrelationID: common.correlation})
	if err != nil {
		return platformSettingFailure(c, err)
	}
	return platformJSON(c, 201, commandVersionJSON(result, "NOT_TESTED", "", responseUsername), nil)
}

func (h *PlatformIntegrationSettingsWriteHandler) test(c fiber.Ctx) error {
	common, versionID, expected, ok := h.versionMutation(c)
	if !ok {
		return nil
	}
	var body settingReasonRequest
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	fields := []string{"reason"}
	if c.Params("setting_kind") == string(domain.OpenAI) {
		fields = append(fields, "confirm_billable_test")
	}
	if !requireWorkflowFields(c.Body(), fields) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if c.Params("setting_kind") == string(domain.OpenAI) && !body.ConfirmBillableTest {
		return fail(c, 400, "INVALID_REQUEST", "Confirm the possible billable test before continuing.", false)
	}
	result, err := h.writer.Test(c.Context(), settingsapp.TestInput{Subject: common.subject, VersionID: versionID, ExpectedVersion: expected, Reason: body.Reason, IdempotencyKey: common.idempotency, CorrelationID: common.correlation})
	if err != nil {
		return platformSettingFailure(c, err)
	}
	status := "FAILED"
	switch result.ResultCode {
	case "SMTP_STARTTLS_VERIFIED", "OPENAI_STRUCTURED_RESPONSE_VERIFIED", "OPENAI_DISABLED_CONFIG_VERIFIED", "PESAPAL_AUTHENTICATION_VERIFIED", "PESAPAL_DISABLED_CONFIG_VERIFIED":
		status = "PASSED"
	}
	return platformJSON(c, 200, commandVersionJSON(result.Version, status, result.ResultCode), nil)
}

func (h *PlatformIntegrationSettingsWriteHandler) propose(c fiber.Ctx) error {
	common, versionID, expected, ok := h.versionMutation(c)
	if !ok {
		return nil
	}
	var body settingProposalRequest
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"action", "reason"}) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	action := domain.Action(body.Action)
	if action != domain.Activate && action != domain.Rollback {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.writer.Propose(c.Context(), settingsapp.ProposeInput{Subject: common.subject, VersionID: versionID, ExpectedVersion: expected, Action: action, Reason: body.Reason, IdempotencyKey: common.idempotency, CorrelationID: common.correlation})
	if err != nil {
		return platformSettingFailure(c, err)
	}
	return platformJSON(c, 201, commandChangeJSON(result), nil)
}

func (h *PlatformIntegrationSettingsWriteHandler) approve(c fiber.Ctx) error {
	return h.decide(c, true)
}
func (h *PlatformIntegrationSettingsWriteHandler) reject(c fiber.Ctx) error {
	return h.decide(c, false)
}
func (h *PlatformIntegrationSettingsWriteHandler) decide(c fiber.Ctx, approve bool) error {
	common, ok := h.mutation(c)
	if !ok {
		return nil
	}
	changeID, ok := settingPathID(c, "change_id")
	if !ok {
		return fail(c, 404, "PLATFORM_SETTING_NOT_FOUND", "The setting change was not found.", false)
	}
	expected, ok := ifMatch(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var body settingReasonRequest
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"reason"}) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.writer.Decide(c.Context(), settingsapp.DecideInput{Subject: common.subject, ChangeID: changeID, ExpectedVersion: expected, Approve: approve, Reason: body.Reason, IdempotencyKey: common.idempotency, CorrelationID: common.correlation})
	if err != nil {
		return platformSettingFailure(c, err)
	}
	change := commandChangeJSON(result)
	if !approve {
		return platformJSON(c, 200, change, nil)
	}
	active, err := h.reader.Active(c.Context(), common.subject, result.Kind)
	if err != nil {
		return platformSettingFailure(c, err)
	}
	return platformJSON(c, 200, map[string]any{"change": change, "active_version": platformSettingVersionJSON(active)}, nil)
}

func (h *PlatformIntegrationSettingsWriteHandler) versionMutation(c fiber.Ctx) (settingMutation, domain.ID, int64, bool) {
	common, ok := h.mutation(c)
	if !ok {
		return common, domain.ID{}, 0, false
	}
	if c.Params("setting_kind") != string(domain.SMTP) && c.Params("setting_kind") != string(domain.OpenAI) && c.Params("setting_kind") != string(domain.PesaPal) {
		_ = fail(c, 404, "PLATFORM_SETTING_NOT_FOUND", "The setting was not found.", false)
		return common, domain.ID{}, 0, false
	}
	id, ok := settingPathID(c, "version_id")
	if !ok {
		_ = fail(c, 404, "PLATFORM_SETTING_NOT_FOUND", "The setting was not found.", false)
		return common, domain.ID{}, 0, false
	}
	expected, ok := ifMatch(c)
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return common, domain.ID{}, 0, false
	}
	return common, id, expected, true
}
func (h *PlatformIntegrationSettingsWriteHandler) mutation(c fiber.Ctx) (settingMutation, bool) {
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return settingMutation{}, false
	}
	generation, ok := platformGeneration(c)
	if !ok {
		return settingMutation{}, false
	}
	raw, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		_ = invalidHeader(c, IdempotencyKeyHeader, "required")
		return settingMutation{}, false
	}
	key, err := uuid.Parse(raw)
	if err != nil || key == uuid.Nil {
		_ = invalidHeader(c, IdempotencyKeyHeader, "uuid")
		return settingMutation{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
		return settingMutation{}, false
	}
	if h.clock.Now().UTC().IsZero() {
		_ = fail(c, 503, "PLATFORM_SETTINGS_UNAVAILABLE", "Platform settings are temporarily unavailable.", true)
		return settingMutation{}, false
	}
	return settingMutation{subject: settingsapp.Subject{ActorID: domain.ID(principal.UserID), SessionID: domain.ID(principal.SessionID), ContextGeneration: generation}, correlation: domain.ID(correlation), idempotency: key.String()}, true
}
func settingPathID(c fiber.Ctx, name string) (domain.ID, bool) {
	parsed, err := uuid.Parse(c.Params(name))
	return domain.ID(parsed), err == nil && parsed != uuid.Nil
}
func commandVersionJSON(v domain.Version, testStatus, testCode string, username ...string) any {
	redacted := settingsapp.RedactedVersion{ID: v.ID, LatestTestID: v.LatestTestID, Kind: v.Kind, Sequence: v.Sequence, State: v.State, Version: v.Version, TestStatus: testStatus, TestCode: testCode, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, ActivatedAt: v.ActivatedAt, SupersededAt: v.SupersededAt}
	if v.Kind == domain.SMTP && v.Configuration.SMTP != nil {
		c := v.Configuration.SMTP
		redacted.SMTP = &settingsapp.RedactedSMTP{Host: c.Host, Port: c.Port, SenderName: c.SenderName, SenderAddress: c.SenderAddress, ReplyToAddress: c.ReplyToAddress, ConnectTimeoutSeconds: c.ConnectTimeoutSeconds, CommandTimeoutSeconds: c.CommandTimeoutSeconds, DeliveryTimeoutSeconds: c.DeliveryTimeoutSeconds, MaxConcurrency: c.MaxConcurrency}
	}
	if redacted.SMTP != nil && len(username) == 1 {
		redacted.SMTP.Username = &username[0]
	}
	if v.Kind == domain.OpenAI && v.Configuration.OpenAI != nil {
		c := v.Configuration.OpenAI
		redacted.OpenAI = &settingsapp.RedactedOpenAI{Enabled: !c.Disabled, Model: c.Model, TimeoutSeconds: c.TimeoutSeconds, MaxConcurrency: c.MaxConcurrency}
	}
	if v.Kind == domain.PesaPal && v.Configuration.PesaPal != nil {
		c := v.Configuration.PesaPal
		redacted.PesaPal = &settingsapp.RedactedPesaPal{Enabled: c.Enabled, Environment: c.Environment, MerchantName: c.MerchantName, NotificationID: c.NotificationID, CredentialsConfigured: c.CredentialsSecretRef != "unconfigured"}
	}
	return platformSettingVersionJSON(redacted)
}
func commandChangeJSON(v domain.Change) any {
	return platformSettingChangeJSON(settingsapp.RedactedChange{ID: v.ID, TargetVersionID: v.TargetVersionID, TargetTestID: v.TargetTestID, ExpectedActiveVersionID: v.ExpectedActiveVersionID, MakerUserID: v.MakerUserID, CheckerUserID: v.CheckerUserID, Kind: v.Kind, Action: v.Action, State: v.State, TargetPriorState: v.TargetPriorState, Version: v.Version, CreatedAt: v.CreatedAt, ExpiresAt: v.ExpiresAt, DecidedAt: v.DecidedAt})
}
