package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bidora/backend/internal/identity/browser"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const platformContextHeader = "X-Bidora-Platform-Context-Generation"

type PlatformAdministrationUseCase interface {
	Roles(context.Context, platformapp.ReadAuthority) ([]platformapp.RoleCatalogueView, error)
	Administrators(context.Context, platformapp.ReadAuthority, platformapp.AdministratorFilter, platformapp.PageCursor) (platformapp.AdministratorList, error)
	Administrator(context.Context, platformapp.ReadAuthority, platformapp.ID) (platformapp.AdministratorView, error)
	Invitation(context.Context, platformapp.ReadAuthority, platformapp.ID) (platformapp.InvitationView, error)
	Invitations(context.Context, platformapp.ReadAuthority, platformapp.PageCursor) (platformapp.InvitationList, error)
	AssignmentHistory(context.Context, platformapp.ReadAuthority, platformapp.ID, platformapp.PageCursor) (platformapp.AssignmentHistory, error)
	Changes(context.Context, platformapp.ReadAuthority, *string, platformapp.PageCursor) (platformapp.ChangeList, error)
	Change(context.Context, platformapp.ReadAuthority, platformapp.ID) (platformapp.ChangeView, error)
	IssueInvitation(context.Context, platformapp.IssueInvitationInput) (platformapp.IssueInvitationResult, error)
	CancelInvitation(context.Context, platformapp.CancelInvitationInput) error
	AcceptInvitation(context.Context, platformapp.AcceptInvitationInput) (platformapp.AcceptInvitationResult, error)
	ChangeRole(context.Context, platformapp.RoleChangeInput) (platformapp.RoleChangeResult, error)
	ChangeIdentityState(context.Context, platformapp.IdentityStateChangeInput) (platformapp.IdentityStateChangeResult, error)
	ApproveOwnerInvitation(context.Context, platformapp.OwnerInvitationDispositionInput) (platformapp.OwnerInvitationDispositionResult, error)
	RejectOwnerInvitation(context.Context, platformapp.OwnerInvitationDispositionInput) (platformapp.OwnerInvitationDispositionResult, error)
	ApproveOwnerRoleChange(context.Context, platformapp.OwnerRoleDispositionInput) (platformapp.OwnerRoleDispositionResult, error)
	RejectOwnerRoleChange(context.Context, platformapp.OwnerRoleDispositionInput) (platformapp.OwnerRoleDispositionResult, error)
	ApproveOwnerIdentityStateChange(context.Context, platformapp.OwnerIdentityDispositionInput) (platformapp.OwnerIdentityDispositionResult, error)
	RejectOwnerIdentityStateChange(context.Context, platformapp.OwnerIdentityDispositionInput) (platformapp.OwnerIdentityDispositionResult, error)
	RevokeAdministratorSessions(context.Context, platformapp.AdministratorSessionRevocationInput) (platformapp.AdministratorSessionRevocationResult, error)
}
type PlatformAdministrationHandler struct {
	usecase PlatformAdministrationUseCase
	clock   BrowserClock
}

func NewPlatformAdministrationHandler(usecase PlatformAdministrationUseCase, clock BrowserClock) (*PlatformAdministrationHandler, error) {
	if usecase == nil || clock == nil {
		return nil, errors.New("platform_administration_handler_invalid")
	}
	return &PlatformAdministrationHandler{usecase, clock}, nil
}

// RegisterPlatformDirectoryRoutes exposes the fixed-role catalogue, bounded
// administrator directory/detail and immutable role-assignment history only.
func RegisterPlatformDirectoryRoutes(app *fiber.App, middleware *BrowserSessionMiddleware, handler *PlatformAdministrationHandler) error {
	if app == nil || middleware == nil || handler == nil {
		return errors.New("platform_administration_routes_invalid")
	}
	g := app.Group("/v1/platform", middleware.Platform)
	g.Get("/roles", handler.roles)
	g.Get("/administrators", handler.administrators)
	g.Post("/administrators/search", handler.searchAdministrators)
	g.Get("/administrators/:user_id", handler.administrator)
	g.Get("/administrators/:user_id/assignment-history", handler.assignmentHistory)
	return nil
}

// RegisterPlatformInvitationRoutes exposes only the Platform Administrator
// invitation journey. Other administrator lifecycle mutations remain unmounted.
func RegisterPlatformInvitationRoutes(app *fiber.App, middleware *BrowserSessionMiddleware, handler *PlatformAdministrationHandler) error {
	if app == nil || middleware == nil || handler == nil {
		return errors.New("platform_administration_routes_invalid")
	}
	g := app.Group("/v1/platform", middleware.Platform)
	g.Get("/administrator-invitations", handler.invitations)
	g.Post("/administrator-invitations", handler.issueInvitation)
	g.Delete("/administrator-invitations/:invitation_id", handler.cancelInvitation)
	g.Post("/administrator-invitations/accept", handler.acceptInvitation)
	return nil
}

// RegisterPlatformAdministratorChangeRoutes exposes the approved role and
// identity-state commands plus the bounded invitation/owner governance queue
// and maker-checker review. Session-only revocation remains unmounted.
func RegisterPlatformAdministratorChangeRoutes(app *fiber.App, middleware *BrowserSessionMiddleware, handler *PlatformAdministrationHandler) error {
	if app == nil || middleware == nil || handler == nil {
		return errors.New("platform_administration_routes_invalid")
	}
	g := app.Group("/v1/platform", middleware.Platform)
	g.Post("/administrator-changes", handler.createChange)
	g.Get("/administrator-changes", handler.changes)
	g.Get("/administrator-changes/:change_id", handler.change)
	g.Post("/administrator-changes/:change_id/approve", handler.approveChange)
	g.Post("/administrator-changes/:change_id/reject", handler.rejectChange)
	return nil
}

// RegisterPlatformSessionRevocationRoute exposes only emergency cross-account
// session revocation. It does not expose session inventory or self-revocation.
func RegisterPlatformSessionRevocationRoute(app *fiber.App, middleware *BrowserSessionMiddleware, handler *PlatformAdministrationHandler) error {
	if app == nil || middleware == nil || handler == nil {
		return errors.New("platform_administration_routes_invalid")
	}
	g := app.Group("/v1/platform", middleware.Platform)
	g.Post("/administrators/:user_id/sessions/revoke", handler.revokeSessions)
	return nil
}

func RegisterPlatformAdministrationRoutes(app *fiber.App, middleware *BrowserSessionMiddleware, handler *PlatformAdministrationHandler) error {
	if app == nil || middleware == nil || handler == nil {
		return errors.New("platform_administration_routes_invalid")
	}
	g := app.Group("/v1/platform", middleware.Platform)
	g.Get("/roles", handler.roles)
	g.Get("/administrator-invitations", handler.invitations)
	g.Post("/administrator-invitations", handler.issueInvitation)
	g.Delete("/administrator-invitations/:invitation_id", handler.cancelInvitation)
	g.Post("/administrator-invitations/accept", handler.acceptInvitation)
	g.Get("/administrators", handler.administrators)
	g.Post("/administrators/search", handler.searchAdministrators)
	g.Get("/administrators/:user_id", handler.administrator)
	g.Get("/administrators/:user_id/assignment-history", handler.assignmentHistory)
	g.Post("/administrator-changes", handler.createChange)
	g.Get("/administrator-changes", handler.changes)
	g.Get("/administrator-changes/:change_id", handler.change)
	g.Post("/administrator-changes/:change_id/approve", handler.approveChange)
	g.Post("/administrator-changes/:change_id/reject", handler.rejectChange)
	g.Post("/administrators/:user_id/sessions/revoke", handler.revokeSessions)
	return nil
}

type platformMeta struct {
	TraceID    string  `json:"trace_id"`
	NextCursor *string `json:"next_cursor,omitempty"`
}
type platformResponse struct {
	Data any          `json:"data"`
	Meta platformMeta `json:"meta"`
}
type platformIssueRequest struct {
	Email   string `json:"email"`
	RoleKey string `json:"role_key"`
	Reason  string `json:"reason"`
}
type platformChangeRequest struct {
	Action                          string `json:"action"`
	TargetUserID                    string `json:"target_user_id"`
	RoleKey                         string `json:"role_key"`
	AssignmentID                    string `json:"assignment_id"`
	Reason                          string `json:"reason"`
	ExpectedUserVersion             int64  `json:"expected_user_version"`
	ExpectedTargetContextGeneration int64  `json:"expected_target_context_generation"`
	ExpectedAssignmentVersion       int64  `json:"expected_assignment_version"`
}

type platformDispositionRequest struct {
	ExpectedChangeVersion int64  `json:"expected_change_version"`
	Reason                string `json:"reason"`
}
type platformSessionRequest struct {
	ExpectedUserVersion             int64  `json:"expected_user_version"`
	ExpectedTargetContextGeneration int64  `json:"expected_target_context_generation"`
	Reason                          string `json:"reason"`
}
type platformAcceptRequest struct {
	Token string `json:"token"`
}

func (h *PlatformAdministrationHandler) roles(c fiber.Ctx) error {
	auth, ok := h.readAuthority(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	rows, err := h.usecase.Roles(c.Context(), auth)
	if err != nil {
		return platformFailure(c, err)
	}
	items := make([]any, len(rows))
	for i, row := range rows {
		items[i] = map[string]any{"role_key": row.Role, "catalogue_version": 1, "capabilities": row.Capabilities}
	}
	return platformJSON(c, 200, map[string]any{"roles": items, "custom_roles_supported": false, "tenant_access_implied": false}, nil)
}
func (h *PlatformAdministrationHandler) administrators(c fiber.Ctx) error {
	auth, ok := h.readAuthority(c)
	if !ok {
		return nil
	}
	page, values, ok := platformPage(c, "state", "role_key")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var filter platformapp.AdministratorFilter
	if v := values.Get("state"); v != "" {
		filter.State = &v
	}
	if v := values.Get("role_key"); v != "" {
		role, e := domain.ParseRole(v)
		if e != nil {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		filter.Role = &role
	}
	result, err := h.usecase.Administrators(c.Context(), auth, filter, page)
	if err != nil {
		return platformFailure(c, err)
	}
	items := make([]any, len(result.Administrators))
	for i, row := range result.Administrators {
		items[i] = administratorJSON(row)
	}
	return platformJSON(c, 200, map[string]any{"administrators": items, "effective_owner_count": result.EffectiveOwnerCount, "continuity_warning": result.ContinuityWarning}, result.NextCursor)
}

// searchAdministrators keeps the mailbox out of URLs and uses the existing
// transaction-scoped directory authority and composition.
func (h *PlatformAdministrationHandler) searchAdministrators(c fiber.Ctx) error {
	if len(c.Request().URI().QueryString()) != 0 || len(c.Body()) < 2 || len(c.Body()) > 512 || strings.Split(c.Get("Content-Type"), ";")[0] != "application/json" {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	var body struct {
		Email   string `json:"email"`
		State   string `json:"state"`
		RoleKey string `json:"role_key"`
	}
	if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	auth, ok := h.authority(c)
	if !ok {
		return nil
	}
	filter := platformapp.AdministratorFilter{Email: &body.Email}
	if body.State != "" {
		filter.State = &body.State
	}
	if body.RoleKey != "" {
		role, parseErr := domain.ParseRole(body.RoleKey)
		if parseErr != nil {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		filter.Role = &role
	}
	result, err := h.usecase.Administrators(c.Context(), auth, filter, platformapp.PageCursor{Limit: 1})
	if err != nil {
		return platformFailure(c, err)
	}
	items := make([]any, len(result.Administrators))
	for i, row := range result.Administrators {
		items[i] = administratorJSON(row)
	}
	return platformJSON(c, 200, map[string]any{"administrators": items, "effective_owner_count": result.EffectiveOwnerCount, "continuity_warning": result.ContinuityWarning}, nil)
}

func (h *PlatformAdministrationHandler) administrator(c fiber.Ctx) error {
	auth, id, ok := h.readTarget(c, "user_id")
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	row, err := h.usecase.Administrator(c.Context(), auth, id)
	if err != nil {
		return platformFailure(c, err)
	}
	return platformJSON(c, 200, administratorJSON(row), nil)
}
func (h *PlatformAdministrationHandler) invitations(c fiber.Ctx) error {
	auth, ok := h.readAuthority(c)
	if !ok {
		return nil
	}
	page, _, ok := platformPage(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.usecase.Invitations(c.Context(), auth, page)
	if err != nil {
		return platformFailure(c, err)
	}
	items := make([]any, len(result.Invitations))
	for i, row := range result.Invitations {
		items[i] = invitationJSON(row)
	}
	return platformJSON(c, 200, map[string]any{"invitations": items}, result.NextCursor)
}
func (h *PlatformAdministrationHandler) assignmentHistory(c fiber.Ctx) error {
	auth, id, ok := h.readTarget(c, "user_id")
	if !ok {
		return nil
	}
	page, _, ok := platformPage(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.usecase.AssignmentHistory(c.Context(), auth, id, page)
	if err != nil {
		return platformFailure(c, err)
	}
	items := make([]any, len(result.Assignments))
	for i, row := range result.Assignments {
		items[i] = assignmentJSON(row)
	}
	return platformJSON(c, 200, map[string]any{"user_id": uuid.UUID(result.UserID).String(), "assignments": items}, result.NextCursor)
}
func (h *PlatformAdministrationHandler) changes(c fiber.Ctx) error {
	auth, ok := h.readAuthority(c)
	if !ok {
		return nil
	}
	page, values, ok := platformPage(c, "state")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var state *string
	if v := values.Get("state"); v != "" {
		state = &v
	}
	result, err := h.usecase.Changes(c.Context(), auth, state, page)
	if err != nil {
		return platformFailure(c, err)
	}
	items := make([]any, len(result.Changes))
	for i, row := range result.Changes {
		items[i] = changeJSON(row)
	}
	return platformJSON(c, 200, map[string]any{"changes": items}, result.NextCursor)
}
func (h *PlatformAdministrationHandler) roleGovernanceChange(c fiber.Ctx) error {
	auth, id, ok := h.readTarget(c, "change_id")
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	row, err := h.usecase.Change(c.Context(), auth, id)
	if err != nil {
		return platformFailure(c, err)
	}
	if row.Action != "ACTIVATE_OWNER_INVITATION" && row.Action != string(platformapp.AssignRole) && row.Action != string(platformapp.RevokeRole) {
		return fail(c, 404, "PLATFORM_CHANGE_NOT_FOUND", "The change was not found.", false)
	}
	return platformJSON(c, 200, changeJSON(row), nil)
}

func (h *PlatformAdministrationHandler) change(c fiber.Ctx) error {
	auth, id, ok := h.readTarget(c, "change_id")
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	row, err := h.usecase.Change(c.Context(), auth, id)
	if err != nil {
		return platformFailure(c, err)
	}
	return platformJSON(c, 200, changeJSON(row), nil)
}

func (h *PlatformAdministrationHandler) issueInvitation(c fiber.Ctx) error {
	common, ok := h.mutation(c)
	if !ok {
		return nil
	}
	var body platformIssueRequest
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	role, err := domain.ParseRole(body.RoleKey)
	if err != nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.usecase.IssueInvitation(c.Context(), platformapp.IssueInvitationInput{ActorID: common.actor, SessionID: common.session, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, PlatformContextGeneration: common.generation, Email: body.Email, Role: role, Reason: body.Reason})
	if err != nil {
		return platformFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	auth := common.read(h.clock.Now())
	row, err := h.usecase.Invitation(c.Context(), auth, result.InvitationID)
	if err != nil {
		return platformFailure(c, err)
	}
	row.Delivery = result.Delivery
	return platformJSON(c, 201, invitationJSON(row), nil)
}
func (h *PlatformAdministrationHandler) cancelInvitation(c fiber.Ctx) error {
	common, ok := h.mutation(c)
	if !ok {
		return nil
	}
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	id, ok := platformPathID(c, "invitation_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	version, ok := ifMatch(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	err := h.usecase.CancelInvitation(c.Context(), platformapp.CancelInvitationInput{ActorID: common.actor, SessionID: common.session, InvitationID: id, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, PlatformContextGeneration: common.generation, ExpectedVersion: version})
	if err != nil {
		return platformFailure(c, err)
	}
	return c.SendStatus(204)
}
func (h *PlatformAdministrationHandler) acceptInvitation(c fiber.Ctx) error {
	common, ok := h.mutationWithoutPlatformContext(c)
	if !ok {
		return nil
	}
	var body platformAcceptRequest
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	result, err := h.usecase.AcceptInvitation(c.Context(), platformapp.AcceptInvitationInput{ActorID: common.actor, SessionID: common.session, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, PresentedToken: body.Token})
	if err != nil {
		return platformFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	setLoginCookie(c, browser.ClearSessionCookie())
	setLoginCookie(c, browser.ClearCSRFCookie())
	return c.SendStatus(204)
}
func (h *PlatformAdministrationHandler) createRoleChange(c fiber.Ctx) error {
	return h.createChangeBounded(c, false)
}
func (h *PlatformAdministrationHandler) createChange(c fiber.Ctx) error {
	return h.createChangeBounded(c, true)
}
func (h *PlatformAdministrationHandler) createChangeBounded(c fiber.Ctx, allowIdentityState bool) error {
	common, ok := h.mutation(c)
	if !ok {
		return nil
	}
	var body platformChangeRequest
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	target, ok := parsePlatformID(body.TargetUserID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var changeID platformapp.ID
	var replay bool
	if body.ExpectedUserVersion < 1 || body.ExpectedTargetContextGeneration < 1 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	switch body.Action {
	case string(platformapp.AssignRole), string(platformapp.RevokeRole):
		if body.Action == string(platformapp.AssignRole) && (body.AssignmentID != "" || body.ExpectedAssignmentVersion != 0) {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		if body.Action == string(platformapp.RevokeRole) && body.ExpectedAssignmentVersion < 1 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		role, e := domain.ParseRole(body.RoleKey)
		if e != nil {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		var assignment platformapp.ID
		if body.Action == string(platformapp.RevokeRole) {
			assignment, ok = parsePlatformID(body.AssignmentID)
			if !ok {
				return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
			}
		}
		r, e := h.usecase.ChangeRole(c.Context(), platformapp.RoleChangeInput{ActorID: common.actor, SessionID: common.session, TargetUserID: target, AssignmentID: assignment, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, PlatformContextGeneration: common.generation, ExpectedAdministratorVersion: body.ExpectedUserVersion, ExpectedTargetContextGeneration: body.ExpectedTargetContextGeneration, ExpectedAssignmentVersion: body.ExpectedAssignmentVersion, Action: platformapp.RoleChangeAction(body.Action), Role: role, Reason: body.Reason})
		if e != nil {
			return platformFailure(c, e)
		}
		changeID, replay = r.ChangeID, r.Replay
	case string(platformapp.DisableAdministrator), string(platformapp.ReactivateAdministrator):
		if !allowIdentityState {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		if body.RoleKey != "" || body.AssignmentID != "" || body.ExpectedAssignmentVersion != 0 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		r, e := h.usecase.ChangeIdentityState(c.Context(), platformapp.IdentityStateChangeInput{ActorID: common.actor, SessionID: common.session, TargetUserID: target, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, PlatformContextGeneration: common.generation, ExpectedAdministratorVersion: body.ExpectedUserVersion, ExpectedTargetContextGeneration: body.ExpectedTargetContextGeneration, Action: platformapp.IdentityStateAction(body.Action), Reason: body.Reason})
		if e != nil {
			return platformFailure(c, e)
		}
		changeID, replay = r.ChangeID, r.Replay
	default:
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if replay {
		c.Set("Idempotency-Replayed", "true")
	}
	row, err := h.usecase.Change(c.Context(), common.read(h.clock.Now()), changeID)
	if err != nil {
		return platformFailure(c, err)
	}
	return platformJSON(c, 201, changeJSON(row), nil)
}
func (h *PlatformAdministrationHandler) approveInvitationChange(c fiber.Ctx) error {
	return h.invitationDisposition(c, true)
}
func (h *PlatformAdministrationHandler) rejectInvitationChange(c fiber.Ctx) error {
	return h.invitationDisposition(c, false)
}
func (h *PlatformAdministrationHandler) invitationDisposition(c fiber.Ctx, approve bool) error {
	common, ok := h.mutation(c)
	if !ok {
		return nil
	}
	id, ok := platformPathID(c, "change_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var body platformDispositionRequest
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	current, err := h.usecase.Change(c.Context(), common.read(h.clock.Now()), id)
	if err != nil {
		return platformFailure(c, err)
	}
	if current.Action != "ACTIVATE_OWNER_INVITATION" {
		return fail(c, 409, "PLATFORM_CHANGE_CONFLICT", "The change cannot be disposed.", false)
	}
	input := platformapp.OwnerInvitationDispositionInput{CheckerID: common.actor, SessionID: common.session, ChangeID: id, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, PlatformContextGeneration: common.generation, ExpectedChangeVersion: body.ExpectedChangeVersion, Reason: body.Reason}
	var result platformapp.OwnerInvitationDispositionResult
	if approve {
		result, err = h.usecase.ApproveOwnerInvitation(c.Context(), input)
	} else {
		result, err = h.usecase.RejectOwnerInvitation(c.Context(), input)
	}
	if err != nil {
		return platformFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	row, err := h.usecase.Change(c.Context(), common.read(h.clock.Now()), id)
	if err != nil {
		return platformFailure(c, err)
	}
	return platformJSON(c, 200, changeJSON(row), nil)
}

func (h *PlatformAdministrationHandler) approveRoleGovernanceChange(c fiber.Ctx) error {
	return h.dispositionBounded(c, true, false)
}
func (h *PlatformAdministrationHandler) rejectRoleGovernanceChange(c fiber.Ctx) error {
	return h.dispositionBounded(c, false, false)
}
func (h *PlatformAdministrationHandler) approveChange(c fiber.Ctx) error {
	return h.disposition(c, true)
}
func (h *PlatformAdministrationHandler) rejectChange(c fiber.Ctx) error {
	return h.disposition(c, false)
}
func (h *PlatformAdministrationHandler) disposition(c fiber.Ctx, approve bool) error {
	return h.dispositionBounded(c, approve, true)
}
func (h *PlatformAdministrationHandler) dispositionBounded(c fiber.Ctx, approve, allowIdentityState bool) error {
	common, ok := h.mutation(c)
	if !ok {
		return nil
	}
	id, ok := platformPathID(c, "change_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var body platformDispositionRequest
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	current, err := h.usecase.Change(c.Context(), common.read(h.clock.Now()), id)
	if err != nil {
		return platformFailure(c, err)
	}
	input := platformapp.OwnerRoleDispositionInput{CheckerID: common.actor, SessionID: common.session, ChangeID: id, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, PlatformContextGeneration: common.generation, ExpectedChangeVersion: body.ExpectedChangeVersion, Reason: body.Reason}
	replay := false
	switch current.Action {
	case "ACTIVATE_OWNER_INVITATION":
		ii := platformapp.OwnerInvitationDispositionInput(input)
		var r platformapp.OwnerInvitationDispositionResult
		if approve {
			r, err = h.usecase.ApproveOwnerInvitation(c.Context(), ii)
		} else {
			r, err = h.usecase.RejectOwnerInvitation(c.Context(), ii)
		}
		replay = r.Replay
	case string(platformapp.AssignRole), string(platformapp.RevokeRole):
		var r platformapp.OwnerRoleDispositionResult
		if approve {
			r, err = h.usecase.ApproveOwnerRoleChange(c.Context(), input)
		} else {
			r, err = h.usecase.RejectOwnerRoleChange(c.Context(), input)
		}
		replay = r.Replay
	case string(platformapp.DisableAdministrator), string(platformapp.ReactivateAdministrator):
		if !allowIdentityState {
			return fail(c, 409, "PLATFORM_CHANGE_CONFLICT", "The change cannot be disposed.", false)
		}
		var r platformapp.OwnerIdentityDispositionResult
		if approve {
			r, err = h.usecase.ApproveOwnerIdentityStateChange(c.Context(), input)
		} else {
			r, err = h.usecase.RejectOwnerIdentityStateChange(c.Context(), input)
		}
		replay = r.Replay
	default:
		return fail(c, 409, "PLATFORM_CHANGE_CONFLICT", "The change cannot be disposed.", false)
	}
	if err != nil {
		return platformFailure(c, err)
	}
	if replay {
		c.Set("Idempotency-Replayed", "true")
	}
	row, err := h.usecase.Change(c.Context(), common.read(h.clock.Now()), id)
	if err != nil {
		return platformFailure(c, err)
	}
	return platformJSON(c, 200, changeJSON(row), nil)
}
func (h *PlatformAdministrationHandler) revokeSessions(c fiber.Ctx) error {
	common, ok := h.mutation(c)
	if !ok {
		return nil
	}
	target, ok := platformPathID(c, "user_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var body platformSessionRequest
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	result, err := h.usecase.RevokeAdministratorSessions(c.Context(), platformapp.AdministratorSessionRevocationInput{ActorID: common.actor, SessionID: common.session, TargetUserID: target, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, PlatformContextGeneration: common.generation, ExpectedAdministratorVersion: body.ExpectedUserVersion, ExpectedTargetContextGeneration: body.ExpectedTargetContextGeneration, Reason: body.Reason})
	if err != nil {
		return platformFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	return c.SendStatus(204)
}

type platformCommon struct {
	actor, session, correlation, idempotency platformapp.ID
	generation                               int64
}

func (v platformCommon) read(now time.Time) platformapp.ReadAuthority {
	return platformapp.ReadAuthority{ActorID: v.actor, SessionID: v.session, PlatformContextGeneration: v.generation, ObservedAt: now.UTC()}
}
func (h *PlatformAdministrationHandler) mutation(c fiber.Ctx) (platformCommon, bool) {
	return h.mutationCommon(c, true)
}
func (h *PlatformAdministrationHandler) mutationWithoutPlatformContext(c fiber.Ctx) (platformCommon, bool) {
	return h.mutationCommon(c, false)
}
func (h *PlatformAdministrationHandler) mutationCommon(c fiber.Ctx, requireGeneration bool) (platformCommon, bool) {
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return platformCommon{}, false
	}
	generation := int64(0)
	if requireGeneration {
		var generationOK bool
		generation, generationOK = platformGeneration(c)
		if !generationOK {
			return platformCommon{}, false
		}
	}
	raw, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		_ = invalidHeader(c, IdempotencyKeyHeader, "required")
		return platformCommon{}, false
	}
	key, err := uuid.Parse(raw)
	if err != nil {
		_ = invalidHeader(c, IdempotencyKeyHeader, "uuid")
		return platformCommon{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
		return platformCommon{}, false
	}
	return platformCommon{actor: platformapp.ID(principal.UserID), session: platformapp.ID(principal.SessionID), correlation: platformapp.ID(correlation), idempotency: platformapp.ID(key), generation: generation}, true
}
func (h *PlatformAdministrationHandler) readAuthority(c fiber.Ctx) (platformapp.ReadAuthority, bool) {
	if len(c.Body()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return platformapp.ReadAuthority{}, false
	}
	return h.authority(c)
}
func (h *PlatformAdministrationHandler) authority(c fiber.Ctx) (platformapp.ReadAuthority, bool) {
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return platformapp.ReadAuthority{}, false
	}
	generation, ok := platformGeneration(c)
	if !ok {
		return platformapp.ReadAuthority{}, false
	}
	now := h.clock.Now().UTC()
	if now.IsZero() {
		_ = fail(c, 503, "PLATFORM_ADMINISTRATION_UNAVAILABLE", "Platform administration is temporarily unavailable.", true)
		return platformapp.ReadAuthority{}, false
	}
	return platformapp.ReadAuthority{ActorID: platformapp.ID(principal.UserID), SessionID: platformapp.ID(principal.SessionID), PlatformContextGeneration: generation, ObservedAt: now}, true
}
func (h *PlatformAdministrationHandler) readTarget(c fiber.Ctx, name string) (platformapp.ReadAuthority, platformapp.ID, bool) {
	auth, ok := h.readAuthority(c)
	if !ok {
		return auth, platformapp.ID{}, false
	}
	id, ok := platformPathID(c, name)
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return auth, id, false
	}
	return auth, id, true
}
func platformGeneration(c fiber.Ctx) (int64, bool) {
	raw, ok := singleHeader(c, platformContextHeader)
	if !ok {
		_ = invalidHeader(c, platformContextHeader, "required")
		return 0, false
	}
	value, ok := positiveInt64(raw)
	if !ok {
		_ = invalidHeader(c, platformContextHeader, "positive_integer")
		return 0, false
	}
	return value, true
}
func platformPathID(c fiber.Ctx, name string) (platformapp.ID, bool) {
	return parsePlatformID(c.Params(name))
}
func parsePlatformID(raw string) (platformapp.ID, bool) {
	id, err := uuid.Parse(raw)
	return platformapp.ID(id), err == nil && id != uuid.Nil
}
func ifMatch(c fiber.Ctx) (int64, bool) {
	raw, ok := singleHeader(c, "If-Match")
	if !ok || len(raw) < 3 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return 0, false
	}
	value, err := strconv.ParseInt(raw[1:len(raw)-1], 10, 64)
	return value, err == nil && value > 0
}

func platformPage(c fiber.Ctx, extra ...string) (platformapp.PageCursor, url.Values, bool) {
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil {
		return platformapp.PageCursor{}, nil, false
	}
	allowed := map[string]bool{"limit": true, "cursor": true}
	for _, key := range extra {
		allowed[key] = true
	}
	for key, list := range values {
		if !allowed[key] || len(list) != 1 {
			return platformapp.PageCursor{}, nil, false
		}
	}
	limit := int32(50)
	if raw := values.Get("limit"); raw != "" {
		parsed, e := strconv.Atoi(raw)
		if e != nil || parsed < 1 || parsed > 100 {
			return platformapp.PageCursor{}, nil, false
		}
		limit = int32(parsed)
	}
	page := platformapp.PageCursor{Limit: limit}
	raw := values.Get("cursor")
	if raw != "" {
		decoded, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil || len(decoded) > 384 {
			return page, nil, false
		}
		var value struct {
			At string `json:"at"`
			ID string `json:"id"`
		}
		if json.Unmarshal(decoded, &value) != nil {
			return page, nil, false
		}
		at, e := time.Parse(time.RFC3339Nano, value.At)
		id, ok := parsePlatformID(value.ID)
		if e != nil || !ok {
			return page, nil, false
		}
		at = at.UTC()
		page.BeforeTime = &at
		page.BeforeID = &id
	}
	return page, values, true
}
func encodePlatformCursor(page *platformapp.PageCursor) (*string, error) {
	if page == nil || page.BeforeTime == nil || page.BeforeID == nil {
		return nil, nil
	}
	raw, err := json.Marshal(struct {
		At string `json:"at"`
		ID string `json:"id"`
	}{page.BeforeTime.UTC().Format(time.RFC3339Nano), uuid.UUID(*page.BeforeID).String()})
	if err != nil {
		return nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	return &value, nil
}
func platformJSON(c fiber.Ctx, status int, data any, next *platformapp.PageCursor) error {
	cursor, err := encodePlatformCursor(next)
	if err != nil {
		return fail(c, 503, "PLATFORM_ADMINISTRATION_UNAVAILABLE", "Platform administration is temporarily unavailable.", true)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(status).JSON(platformResponse{Data: data, Meta: platformMeta{TraceID: c.GetRespHeader("X-Request-ID"), NextCursor: cursor}})
}
func platformFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, platformapp.ErrInvitationInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(err, platformapp.ErrPlatformAccessForbidden):
		return fail(c, 403, "PLATFORM_ACCESS_FORBIDDEN", "Platform administration access is forbidden.", false)
	case errors.Is(err, platformapp.ErrPlatformAssuranceRequired):
		return fail(c, 403, "PLATFORM_ASSURANCE_REQUIRED", "Recent administrator verification is required.", false)
	case errors.Is(err, platformapp.ErrPlatformContextStale):
		return fail(c, 409, "PLATFORM_CONTEXT_STALE", "The platform context is stale.", false)
	case errors.Is(err, platformapp.ErrAuthorityNotFound), errors.Is(err, platformapp.ErrInvitationNotFound):
		return fail(c, 404, "PLATFORM_RESOURCE_NOT_FOUND", "The resource was not found.", false)
	case errors.Is(err, platformapp.ErrConcurrentChange), errors.Is(err, platformapp.ErrInvitationConflict):
		return fail(c, 409, "PLATFORM_CHANGE_CONFLICT", "The resource changed concurrently.", false)
	default:
		return fail(c, 503, "PLATFORM_ADMINISTRATION_UNAVAILABLE", "Platform administration is temporarily unavailable.", true)
	}
}

func administratorJSON(row platformapp.AdministratorView) any {
	assignments := make([]any, len(row.Assignments))
	for i, a := range row.Assignments {
		assignments[i] = assignmentJSON(a)
	}
	state := "ACTIVE"
	if row.Identity.Disabled {
		state = "DISABLED"
	}
	return map[string]any{"user_id": uuid.UUID(row.Profile.UserID).String(), "email": row.Identity.Email, "state": state, "user_version": row.Profile.Version, "platform_context_generation": row.Profile.ContextGeneration, "totp_enrolled": row.Identity.TOTPEnrolled, "assignments": assignments, "effective_owner": row.EffectiveOwner, "created_at": row.Profile.CreatedAt, "updated_at": row.Profile.UpdatedAt}
}
func assignmentJSON(row platformapp.AssignmentView) any {
	return map[string]any{"id": uuid.UUID(row.ID).String(), "role_key": row.Role, "state": row.State, "version": row.Version, "assigned_by": platformOptionalID(row.AssignedBy), "assigned_at": row.AssignedAt, "revoked_by": platformOptionalID(row.RevokedBy), "revoked_at": row.RevokedAt, "reason": row.Reason, "revocation_reason": row.RevocationReason}
}
func invitationJSON(row platformapp.InvitationView) any {
	return map[string]any{"id": uuid.UUID(row.ID).String(), "email": row.Email, "role_key": row.Role, "state": row.State, "version": row.Version, "created_at": row.CreatedAt, "expires_at": row.ExpiresAt, "delivery_state": row.Delivery, "governance_change_id": platformOptionalID(row.GovernanceChangeID)}
}
func changeJSON(row platformapp.ChangeView) any {
	var role any
	if row.Role != nil {
		role = *row.Role
	}
	return map[string]any{"id": uuid.UUID(row.ID).String(), "action": row.Action, "target_user_id": platformOptionalID(row.TargetUserID), "invitation_id": platformOptionalID(row.InvitationID), "role_key": role, "assignment_id": platformOptionalID(row.AssignmentID), "expected_user_version": row.ExpectedAdministratorVersion, "expected_target_context_generation": row.ExpectedContextGeneration, "expected_assignment_version": row.ExpectedAssignmentVersion, "reason": row.Reason, "decision_reason": row.DecisionReason, "state": row.State, "version": row.Version, "maker_id": uuid.UUID(row.MakerID).String(), "checker_id": platformOptionalID(row.CheckerID), "created_at": row.CreatedAt, "expires_at": row.ExpiresAt, "decided_at": row.DecidedAt, "sessions_revoked": row.SessionsRevoked}
}
func platformOptionalID(id *platformapp.ID) any {
	if id == nil {
		return nil
	}
	return uuid.UUID(*id).String()
}
