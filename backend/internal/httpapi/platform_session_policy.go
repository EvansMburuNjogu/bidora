package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"bidora/backend/internal/identity/securitypolicy"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformSessionPolicyUseCase interface {
	Read(context.Context, app.Subject) (app.SessionPolicy, error)
	Save(context.Context, app.SessionPolicySave) (app.SessionPolicy, error)
}
type PlatformSessionPolicyHandler struct{ service PlatformSessionPolicyUseCase }

func NewPlatformSessionPolicyHandler(service PlatformSessionPolicyUseCase) (*PlatformSessionPolicyHandler, error) {
	if service == nil {
		return nil, errors.New("session_policy_handler_invalid")
	}
	return &PlatformSessionPolicyHandler{service}, nil
}
func RegisterPlatformSessionPolicyRoutes(f *fiber.App, b *BrowserSessionMiddleware, h *PlatformSessionPolicyHandler) error {
	if f == nil || b == nil || h == nil {
		return errors.New("session_policy_routes_invalid")
	}
	f.Get("/v1/auth/password-policy", func(c fiber.Ctx) error {
        if len(c.Body())!=0||len(c.Request().URI().QueryString())!=0{return fail(c,400,"INVALID_REQUEST","The request is invalid.",false)}
		p := securitypolicy.From(c.Context())
		return platformJSON(c, 200, fiber.Map{"minimum_password_bytes": p.MinimumPasswordBytes, "maximum_password_bytes": 128}, nil)
	})
	f.Get("/v1/platform/security/session-policy", b.Platform, h.read)
	f.Put("/v1/platform/security/session-policy", b.Platform, h.save)
	return nil
}
func (h *PlatformSessionPolicyHandler) read(c fiber.Ctx) error {
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return nil
	}
	p, e := h.service.Read(c.Context(), subject)
	if e != nil {
		return informationFailure(c, e)
	}
	return sessionPolicyJSON(c, p)
}
func sessionPolicyJSON(c fiber.Ctx, p app.SessionPolicy) error {
	c.Set("ETag", fmt.Sprintf("\"%d\"", p.Version))
	return platformJSON(c, 200, fiber.Map{"idle_seconds": p.IdleSeconds, "absolute_seconds": p.AbsoluteSeconds, "version": p.Version, "security": p.Security}, nil)
}
func (h *PlatformSessionPolicyHandler) save(c fiber.Ctx) error {
	if len(c.Body()) < 2 || len(c.Body()) > 4096 || len(c.Request().URI().QueryString()) != 0 || strings.Split(c.Get("Content-Type"), ";")[0] != "application/json" {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return nil
	}
	version, ok := ifMatch(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	key, ok := singleHeader(c, IdempotencyKeyHeader)
	id, e := uuid.Parse(key)
	if !ok || e != nil || id == uuid.Nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return informationFailure(c, app.ErrInformationUnavailable)
	}
	p := app.SessionPolicy{Version: version}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	token, e := decoder.Token()
	if e != nil || token != json.Delim('{') {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, e = decoder.Token()
		name, valid := token.(string)
		if e != nil || !valid || seen[name] {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		seen[name] = true
		switch name {
		case "idle_seconds":
			e = decoder.Decode(&p.IdleSeconds)
		case "absolute_seconds":
			e = decoder.Decode(&p.AbsoluteSeconds)
		case "security":
			e = decodeSecurityControls(decoder, &p.Security)
		default:
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		if e != nil {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
	}
	token, e = decoder.Token()
	if e != nil || token != json.Delim('}') || len(seen) != 3 || !securitypolicy.ValidControls(p.Security) || decoder.Decode(new(any)) != io.EOF {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	p, e = h.service.Save(c.Context(), app.SessionPolicySave{Subject: subject, Policy: p, IdempotencyKey: id.String(), CorrelationID: domain.ID(correlation)})
	if e != nil {
		return informationFailure(c, e)
	}
	return sessionPolicyJSON(c, p)
}
